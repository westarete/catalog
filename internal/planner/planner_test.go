package planner

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func TestPlanQuietPeriod(t *testing.T) {
	tests := []struct {
		name      string
		changedAt time.Time
		wantIn    bool
	}{
		{"changed 2m59s ago is still quiet", now.Add(-2*time.Minute - 59*time.Second), false},
		{"changed exactly 3m ago is ready", now.Add(-3 * time.Minute), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := []Candidate{{Dir: "a", Rel: "x.md", ChangedAt: tt.changedAt}}

			got := Plan(cs, now)

			if gotIn := len(got) == 1; gotIn != tt.wantIn {
				t.Errorf("Plan() kept candidate = %v, want %v", gotIn, tt.wantIn)
			}
		})
	}
}

func TestPlanSkipsCurrent(t *testing.T) {
	cs := []Candidate{
		{Dir: "a", Rel: "x.md", ChangedAt: now.Add(-time.Hour), HasCurrent: true},
	}

	if got := Plan(cs, now); len(got) != 0 {
		t.Errorf("Plan() = %+v, want no candidates", got)
	}
}

func TestPlanRespectsRetry(t *testing.T) {
	tests := []struct {
		name    string
		retryAt time.Time
		wantIn  bool
	}{
		{"retry one second in the future is excluded", now.Add(time.Second), false},
		{"retry equal to now is included", now, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := []Candidate{{
				Dir:       "a",
				Rel:       "x.md",
				ChangedAt: now.Add(-time.Hour),
				RetryAt:   tt.retryAt,
			}}

			got := Plan(cs, now)

			if gotIn := len(got) == 1; gotIn != tt.wantIn {
				t.Errorf("Plan() kept candidate = %v, want %v", gotIn, tt.wantIn)
			}
		})
	}
}

func TestPlanOrder(t *testing.T) {
	// "new" has no summary and changed recently. "old" has a summary that is
	// out of date and changed long ago. The file without a summary goes first,
	// even though it changed more recently.
	recent := now.Add(-10 * time.Minute)
	long := now.Add(-10 * time.Hour)
	cs := []Candidate{
		{Dir: "a", Rel: "old.md", ChangedAt: long, HasOlder: true},
		{Dir: "a", Rel: "new.md", ChangedAt: recent},
	}

	got := Plan(cs, now)

	want := []Candidate{cs[1], cs[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Plan() order = %+v, want %+v", got, want)
	}
}

func TestPlanOrderWithinGroup(t *testing.T) {
	// Both candidates lack a summary. The one that changed earlier goes first.
	cs := []Candidate{
		{Dir: "a", Rel: "later.md", ChangedAt: now.Add(-5 * time.Minute)},
		{Dir: "a", Rel: "earlier.md", ChangedAt: now.Add(-20 * time.Minute)},
	}

	got := Plan(cs, now)

	want := []Candidate{cs[1], cs[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Plan() order = %+v, want %+v", got, want)
	}
}

func TestPlanTiesBreakByDirThenRel(t *testing.T) {
	changed := now.Add(-time.Hour)
	cs := []Candidate{
		{Dir: "b", Rel: "a.md", ChangedAt: changed},
		{Dir: "a", Rel: "z.md", ChangedAt: changed},
		{Dir: "a", Rel: "b.md", ChangedAt: changed},
	}

	got := Plan(cs, now)

	want := []Candidate{cs[2], cs[1], cs[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Plan() order = %+v, want %+v", got, want)
	}
}

func TestPlanDoesNotModifyInput(t *testing.T) {
	changed := now.Add(-time.Hour)
	cs := []Candidate{
		{Dir: "a", Rel: "new.md", ChangedAt: changed},
		{Dir: "a", Rel: "old.md", ChangedAt: changed, HasOlder: true},
	}
	before := append([]Candidate(nil), cs...)

	Plan(cs, now)

	if !reflect.DeepEqual(cs, before) {
		t.Errorf("Plan() changed its input: got %+v, want %+v", cs, before)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{0, 0},
		{1, time.Minute},
		{2, 2 * time.Minute},
		{3, 4 * time.Minute},
		{9, 256 * time.Minute},
		{10, 6 * time.Hour},
		{50, 6 * time.Hour},
		// Without the cap inside the loop, doubling would overflow
		// time.Duration long before 1000 failures.
		{1000, 6 * time.Hour},
	}
	for _, tt := range tests {
		if got := Backoff(tt.failures); got != tt.want {
			t.Errorf("Backoff(%d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

// A partial summary is updated only once no other file in its catalog is
// waiting for a first summary, so the model sees every neighbor it missed.
func TestPlanHoldsPartialUpdateWhileANeighborWaits(t *testing.T) {
	old := now.Add(-time.Hour)
	partial := Candidate{Dir: "a", Rel: "x.md", Hash: "h", ChangedAt: old, HasOlder: true, HasPartial: true}
	tests := []struct {
		name   string
		other  Candidate
		wantIn bool
	}{
		{"neighbor still settling", Candidate{Dir: "a", Rel: "y.md", ChangedAt: now}, false},
		{"neighbor summarized", Candidate{Dir: "a", Rel: "y.md", ChangedAt: old, HasCurrent: true, HasOlder: true}, true},
		{"neighbor has an older summary", Candidate{Dir: "a", Rel: "y.md", ChangedAt: now, HasOlder: true}, true},
		{"neighbor failed", Candidate{Dir: "a", Rel: "y.md", ChangedAt: old, RetryAt: now.Add(time.Hour)}, true},
		{"waiting file in another catalog", Candidate{Dir: "b", Rel: "y.md", ChangedAt: now}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Plan([]Candidate{partial, tt.other}, now)

			if gotIn := slices.ContainsFunc(got, func(c Candidate) bool { return c.Hash == "h" }); gotIn != tt.wantIn {
				t.Errorf("Plan() kept the partial update = %v, want %v", gotIn, tt.wantIn)
			}
		})
	}
}

// Copies of a file share one summary, so a copy in a catalog with nothing
// waiting is held too; its update would see the wrong neighbors.
func TestPlanHoldsPartialUpdateOfEveryCopy(t *testing.T) {
	old := now.Add(-time.Hour)
	cs := []Candidate{
		{Dir: "a", Rel: "x.md", Hash: "h", ChangedAt: old, HasOlder: true, HasPartial: true},
		{Dir: "a", Rel: "w.md", Hash: "w", ChangedAt: now},
		{Dir: "b", Rel: "x.md", Hash: "h", ChangedAt: old, HasOlder: true, HasPartial: true},
	}

	if got := Plan(cs, now); len(got) != 0 {
		t.Errorf("Plan() = %+v, want nothing ready", got)
	}
}

func TestWouldBePartial(t *testing.T) {
	waiting := Candidate{Dir: "a", Rel: "y.md", Hash: "y", ChangedAt: now}
	tests := []struct {
		name string
		c    Candidate
		cs   []Candidate
		want bool
	}{
		{"first summary while a neighbor waits", Candidate{Dir: "a", Rel: "x.md", Hash: "x"}, []Candidate{waiting}, true},
		{"first summary with every neighbor summarized", Candidate{Dir: "a", Rel: "x.md", Hash: "x"},
			[]Candidate{{Dir: "a", Rel: "y.md", HasCurrent: true, HasOlder: true}}, false},
		{"the file itself doesn't count", Candidate{Dir: "a", Rel: "y.md", Hash: "y"}, []Candidate{waiting}, false},
		{"an identical copy doesn't count", Candidate{Dir: "a", Rel: "copy.md", Hash: "y"}, []Candidate{waiting}, false},
		{"an update is never partial", Candidate{Dir: "a", Rel: "x.md", Hash: "x", HasOlder: true}, []Candidate{waiting}, false},
		{"a neighbor with a failure doesn't count", Candidate{Dir: "a", Rel: "x.md", Hash: "x"},
			[]Candidate{{Dir: "a", Rel: "y.md", RetryAt: now}}, false},
		{"a waiting file in another catalog doesn't count", Candidate{Dir: "b", Rel: "x.md", Hash: "x"}, []Candidate{waiting}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WouldBePartial(tt.cs, tt.c); got != tt.want {
				t.Errorf("WouldBePartial() = %v, want %v", got, tt.want)
			}
		})
	}
}
