// Package journal appends daemon events to journal.jsonl, one JSON object per line.
package journal

import (
	"encoding/json"
	"os"
	"time"
)

// Kinds of event the daemon records.
const (
	KindSummary      = "summary"
	KindFailure      = "failure"
	KindPause        = "pause"
	KindResume       = "resume"
	KindConfigError  = "config_error"
	KindPublishError = "publish_error"
)

// Event is one line in the journal. Fields that do not apply to a kind are
// left zero and omitted from the JSON.
type Event struct {
	Time         time.Time `json:"time"`
	Kind         string    `json:"kind"`
	Dir          string    `json:"dir,omitempty"`
	Rel          string    `json:"rel,omitempty"`
	Recipe       string    `json:"recipe,omitempty"`
	Error        string    `json:"error,omitempty"`
	InputTokens  int64     `json:"input_tokens,omitempty"`
	OutputTokens int64     `json:"output_tokens,omitempty"`
	Millis       int64     `json:"millis,omitempty"`
}

// Append writes e as one JSON line at the end of the file at path, creating
// the file with mode 0644 if it does not exist. It never truncates the file,
// so earlier events survive. It does not create missing directories.
func Append(path string, e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
