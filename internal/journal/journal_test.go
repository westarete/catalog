package journal

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func TestAppendLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	first := Event{
		Time:         time.Date(2026, 11, 15, 9, 30, 0, 0, time.UTC),
		Kind:         KindSummary,
		Dir:          "/notes",
		Rel:          "a.md",
		Recipe:       "claude-haiku-5-5",
		InputTokens:  1200,
		OutputTokens: 80,
		Millis:       950,
	}
	second := Event{
		Time:   time.Date(2026, 11, 15, 9, 31, 0, 0, time.UTC),
		Kind:   KindFailure,
		Dir:    "/notes",
		Rel:    "b.pdf",
		Error:  "rate limited",
		Millis: 12,
	}

	if err := Append(path, first); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, second); err != nil {
		t.Fatal(err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), lines)
	}
	want := []Event{first, second}
	for i, line := range lines {
		var got Event
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if !got.Time.Equal(want[i].Time) || !reflect.DeepEqual(withoutTime(got), withoutTime(want[i])) {
			t.Errorf("line %d decoded to %+v, want %+v", i, got, want[i])
		}
	}
}

func withoutTime(e Event) Event {
	e.Time = time.Time{}
	return e
}

func TestAppendCreatesFileWithMode0644(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := Append(path, Event{Kind: KindPause}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestAppendKeepsExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := os.WriteFile(path, []byte("{\"kind\":\"old\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, Event{Kind: KindResume}); err != nil {
		t.Fatal(err)
	}
	lines := readLines(t, path)
	if len(lines) != 2 || lines[0] != `{"kind":"old"}` {
		t.Fatalf("lines = %q; want the old line kept and one new line", lines)
	}
}

func TestAppendEndsEachLineWithNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := Append(path, Event{Kind: KindPause}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("file = %q; want a trailing newline", data)
	}
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("file = %q; want exactly one line", data)
	}
}

func TestAppendOmitsEmptyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := Append(path, Event{Kind: KindConfigError, Error: "bad toml"}); err != nil {
		t.Fatal(err)
	}
	line := readLines(t, path)[0]
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dir", "rel", "recipe", "input_tokens", "output_tokens", "millis"} {
		if _, ok := fields[key]; ok {
			t.Errorf("field %q present; want omitted when empty: %s", key, line)
		}
	}
	if fields["kind"] != "config_error" || fields["error"] != "bad toml" {
		t.Errorf("line = %s; want kind and error set", line)
	}
}

func TestAppendUsesSnakeCaseKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	e := Event{Kind: KindSummary, InputTokens: 7, OutputTokens: 3, Millis: 5}
	if err := Append(path, e); err != nil {
		t.Fatal(err)
	}
	line := readLines(t, path)[0]
	for _, key := range []string{`"input_tokens":7`, `"output_tokens":3`, `"millis":5`, `"kind":"summary"`} {
		if !strings.Contains(line, key) {
			t.Errorf("line = %s; want it to contain %s", line, key)
		}
	}
}

func TestAppendReturnsErrorWhenDirMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "journal.jsonl")
	if err := Append(path, Event{Kind: KindPause}); err == nil {
		t.Fatal("Append into a missing directory returned nil; want an error")
	}
}

// TestAppendReturnsWriteError lowers the file-size limit so the write fails
// with EFBIG. The limit is restored when the test ends. Tests in this package
// do not run in parallel, so the change does not reach other tests.
func TestAppendReturnsWriteError(t *testing.T) {
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 1, Max: old.Max}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old) })

	path := filepath.Join(t.TempDir(), "journal.jsonl")
	if err := Append(path, Event{Kind: KindPause}); err == nil {
		t.Fatal("Append returned nil after a failed write; want an error")
	}
}

func TestKindValues(t *testing.T) {
	got := map[string]string{
		KindSummary:      "summary",
		KindFailure:      "failure",
		KindPause:        "pause",
		KindResume:       "resume",
		KindConfigError:  "config_error",
		KindPublishError: "publish_error",
	}
	for kind, want := range got {
		if kind != want {
			t.Errorf("kind = %q, want %q", kind, want)
		}
	}
}
