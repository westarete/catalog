package planner

import (
	"reflect"
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
