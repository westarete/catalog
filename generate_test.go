package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func TestTokensAdd(t *testing.T) {
	got := tokens{input: 1, cached: 2, output: 3}.add(tokens{input: 10, cached: 20, output: 30})
	if got != (tokens{input: 11, cached: 22, output: 33}) {
		t.Errorf("add = %+v", got)
	}
}

func TestFormatRunSummary(t *testing.T) {
	s := formatRunSummary(20, 2, tokens{input: 1000, cached: 500, output: 200}, 90*time.Second)
	for _, want := range []string{"20 docs", "2 pass", "40 calls", "1000 input", "500 cached", "200 output", "90s"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary missing %q: %s", want, s)
		}
	}
}

func TestRequirePopulated(t *testing.T) {
	// Empty store: refuse and point at bootstrap.
	err := requirePopulated(nil)
	if err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("want error pointing at bootstrap, got %v", err)
	}
	// A populated store passes.
	if err := requirePopulated([]profileRow{{path: "a.md", contentHash: "h", profile: "p"}}); err != nil {
		t.Fatalf("populated store should pass, got %v", err)
	}
}

func TestResolveForceTargets(t *testing.T) {
	docs := []string{"a.md", "b.md", "c.md"}

	// Named files come back in docs order, not the order they were passed.
	got, err := resolveForceTargets([]string{"c.md", "a.md"}, docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a.md", "c.md"}) {
		t.Errorf("targets = %v, want [a.md c.md]", got)
	}

	// A name that isn't enumerated is an error naming the config, not a no-op.
	_, err = resolveForceTargets([]string{"a.md", "typo.md"}, docs)
	if err == nil || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("want error naming %s, got %v", configPath, err)
	}
}

func TestBuildProfilePrompt(t *testing.T) {
	// No neighbors: just the document tag, no positioning section.
	solo := buildProfilePrompt("a.md", "body text", "")
	if !strings.Contains(solo, `<document path="a.md">`) || !strings.Contains(solo, "body text") {
		t.Errorf("missing document tag/body:\n%s", solo)
	}
	if strings.Contains(solo, "Existing profiles") {
		t.Errorf("solo prompt should have no neighbor section:\n%s", solo)
	}
	// With neighbors: positioning section appended.
	withN := buildProfilePrompt("a.md", "body", "- b.md: when X\n")
	if !strings.Contains(withN, "Existing profiles for other documents") {
		t.Errorf("missing neighbor section:\n%s", withN)
	}
	if !strings.Contains(withN, "- b.md: when X") {
		t.Errorf("missing neighbor line:\n%s", withN)
	}
}

func TestInferProfileReportsRefusalDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-test",
			"content":     []any{},
			"stop_reason": "refusal",
			"stop_details": map[string]any{
				"type":        "refusal",
				"category":    "cyber",
				"explanation": "the request resembled exploit content",
			},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 0},
		})
	}))
	defer srv.Close()

	client := anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"))
	cfg := &config{Model: "claude-test"}
	_, _, err := inferProfile(context.Background(), &client, cfg, "a.md", "body", "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "cyber") || !strings.Contains(err.Error(), "exploit content") {
		t.Errorf("error should surface refusal category/explanation, got: %v", err)
	}
}

func TestNeighborsExcludesSelfAndSorts(t *testing.T) {
	db, err := openStore(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, r := range []profileRow{
		{path: "c.md", contentHash: "h", profile: "pc"},
		{path: "a.md", contentHash: "h", profile: "pa"},
		{path: "b.md", contentHash: "h", profile: "pb"},
	} {
		if err := writeProfile(db, r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := neighbors(db, "b.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "- a.md: pa\n- c.md: pc\n"
	if got != want {
		t.Errorf("neighbors = %q, want %q", got, want)
	}
}

func TestHashDocs(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.md")
	if err := os.WriteFile(pathA, []byte("content A"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := hashDocs([]string{pathA})
	if err != nil {
		t.Fatal(err)
	}
	want := contentHash([]byte("content A"))
	if got[pathA] != want {
		t.Errorf("hashDocs[%s] = %s, want %s", pathA, got[pathA], want)
	}
}
