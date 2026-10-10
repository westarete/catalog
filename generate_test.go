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
	"unicode/utf8"

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

func TestFormatDeletedReport(t *testing.T) {
	if got := formatDeletedReport(nil); got != "" {
		t.Errorf("no deletions: got %q, want empty", got)
	}
	got := formatDeletedReport([]string{"a.md", "docs/b.md"})
	want := "catalog: removed 2 deleted doc(s):\n  a.md\n  docs/b.md"
	if got != want {
		t.Errorf("report = %q, want %q", got, want)
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

func TestIsPDF(t *testing.T) {
	cases := map[string]bool{
		"a.pdf":              true,
		"A.PDF":              true,
		"reports/annual.pdf": true,
		"a.md":               false,
		"a.txt":              false,
		"a.pdf.md":           false,
	}
	for path, want := range cases {
		if got := isPDF(path); got != want {
			t.Errorf("isPDF(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestBuildDocumentBlocksTextPath(t *testing.T) {
	blocks := buildDocumentBlocks("a.md", []byte("body text"), "- b.md: when X\n")
	if len(blocks) != 1 {
		t.Fatalf("want 1 block for a text document, got %d", len(blocks))
	}
	text := blocks[0].OfText
	if text == nil {
		t.Fatal("want a text block")
	}
	if !strings.Contains(text.Text, `<document path="a.md">`) || !strings.Contains(text.Text, "body text") {
		t.Errorf("missing document tag/body:\n%s", text.Text)
	}
	if !strings.Contains(text.Text, "- b.md: when X") {
		t.Errorf("missing neighbor line:\n%s", text.Text)
	}
}

func TestTruncateDocument(t *testing.T) {
	t.Run("within the limit is unchanged", func(t *testing.T) {
		if got := truncateDocument("a\nb\n", 100); got != "a\nb\n" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("cuts at a line boundary and says so", func(t *testing.T) {
		got := truncateDocument("row1\nrow2\nrow3\n", 12)
		if !strings.HasPrefix(got, "row1\nrow2\n\n[Document truncated") {
			t.Errorf("want cut after row2, got %q", got)
		}
		if strings.Contains(got, "row3") {
			t.Errorf("row3 should be cut: %q", got)
		}
		if !strings.Contains(got, "first 9 of 15 bytes") {
			t.Errorf("note should give the byte counts: %q", got)
		}
	})
	t.Run("a single long line is cut on a character boundary", func(t *testing.T) {
		got := truncateDocument("éééééé", 5) // each é is 2 bytes
		if !utf8.ValidString(got) {
			t.Errorf("cut split a character: %q", got)
		}
		if !strings.HasPrefix(got, "éé\n\n[Document truncated") {
			t.Errorf("got %q", got)
		}
	})
}

func TestBuildDocumentBlocksTruncatesHugeText(t *testing.T) {
	huge := strings.Repeat("a,b,c\n", maxDocumentBytes) // 6x the cap
	blocks := buildDocumentBlocks("big.csv", []byte(huge), "")
	text := blocks[0].OfText.Text
	if len(text) > maxDocumentBytes+1000 {
		t.Errorf("prompt is %d bytes, want about %d", len(text), maxDocumentBytes)
	}
	if !strings.Contains(text, "[Document truncated") {
		t.Error("prompt should say the document was truncated")
	}
}

func TestBuildDocumentBlocksPDFPath(t *testing.T) {
	blocks := buildDocumentBlocks("a.pdf", []byte("%PDF-1.4 fake content"), "- b.md: when X\n")
	if len(blocks) != 2 {
		t.Fatalf("want a document block plus a neighbor text block, got %d blocks", len(blocks))
	}
	doc := blocks[0].OfDocument
	if doc == nil {
		t.Fatal("want the first block to be a document block")
	}
	pdfSource := doc.Source.OfBase64
	if pdfSource == nil {
		t.Fatal("want a base64 PDF source")
	}
	wantData := "JVBERi0xLjQgZmFrZSBjb250ZW50" // base64 of "%PDF-1.4 fake content"
	if pdfSource.Data != wantData {
		t.Errorf("PDF data = %q, want %q", pdfSource.Data, wantData)
	}
	text := blocks[1].OfText
	if text == nil {
		t.Fatal("want the second block to be a text block")
	}
	if !strings.Contains(text.Text, "Existing profiles for other documents") {
		t.Errorf("missing neighbor section:\n%s", text.Text)
	}
	if !strings.Contains(text.Text, "- b.md: when X") {
		t.Errorf("missing neighbor line:\n%s", text.Text)
	}
	if strings.Contains(text.Text, "<document path=") {
		t.Errorf("PDF path should not also inline a <document> tag:\n%s", text.Text)
	}
}

func TestBuildDocumentBlocksPDFPathNoNeighbors(t *testing.T) {
	blocks := buildDocumentBlocks("a.pdf", []byte("content"), "")
	if len(blocks) != 1 {
		t.Fatalf("want just the document block when there are no neighbors, got %d blocks", len(blocks))
	}
	if blocks[0].OfDocument == nil {
		t.Fatal("want a document block")
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
	_, _, err := inferProfile(context.Background(), &client, cfg, "a.md", []byte("body"), "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "cyber") || !strings.Contains(err.Error(), "exploit content") {
		t.Errorf("error should surface refusal category/explanation, got: %v", err)
	}
}

// A 1024-token cap let a thinking model spend the whole response thinking and
// return no profile text. This pins the request to the roomier cap.
func TestInferProfileLeavesRoomForThinking(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-test",
			"content":     []any{map[string]any{"type": "text", "text": "a profile"}},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer srv.Close()

	client := anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"))
	cfg := &config{Model: "claude-test"}
	if _, _, err := inferProfile(context.Background(), &client, cfg, "a.md", []byte("body"), ""); err != nil {
		t.Fatal(err)
	}
	if got := gotBody["max_tokens"]; got != float64(profileMaxTokens) {
		t.Errorf("max_tokens = %v, want %d", got, profileMaxTokens)
	}
}

func TestInferProfileSendsDocumentBlockForPDF(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-test",
			"content":     []any{map[string]any{"type": "text", "text": "a profile"}},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer srv.Close()

	client := anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"))
	cfg := &config{Model: "claude-test"}
	_, _, err := inferProfile(context.Background(), &client, cfg, "a.pdf", []byte("%PDF-1.4 fake"), "")
	if err != nil {
		t.Fatal(err)
	}

	messages, _ := gotBody["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(messages))
	}
	content, _ := messages[0].(map[string]any)["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("want 1 content block for a PDF with no neighbors, got %d", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "document" {
		t.Fatalf("want a document block on the wire, got type %v", block["type"])
	}
	source, _ := block["source"].(map[string]any)
	if source["media_type"] != "application/pdf" {
		t.Errorf("want application/pdf media type, got %v", source["media_type"])
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

// TestInferPassSkipsFailuresAndKeepsGoing verifies that one document's
// inference failure (e.g. the model exhausted MaxTokens before producing
// profile text) does not stop the pass from writing every other document's
// profile — it should be reported, not fatal to the whole batch.
func TestInferPassSkipsFailuresAndKeepsGoing(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.md")
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(good, []byte("good content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("bad content"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages, _ := body["messages"].([]any)
		msg, _ := messages[0].(map[string]any)
		content, _ := msg["content"].([]any)
		block, _ := content[0].(map[string]any)
		text, _ := block["text"].(string)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(text, "bad content") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "msg_test", "type": "message", "role": "assistant", "model": "claude-test",
				"content": []any{}, "stop_reason": "max_tokens",
				"usage": map[string]any{"input_tokens": 1, "output_tokens": 400},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_test", "type": "message", "role": "assistant", "model": "claude-test",
			"content": []any{map[string]any{"type": "text", "text": "a good profile"}}, "stop_reason": "end_turn",
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 3},
		})
	}))
	defer srv.Close()

	db, err := openStore(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	client := anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"))
	cfg := &config{Model: "claude-test"}

	_, err = inferPass(context.Background(), db, &client, cfg, []string{bad, good})
	if err == nil {
		t.Fatal("want an error naming the failed document, got nil")
	}
	if !strings.Contains(err.Error(), bad) {
		t.Errorf("error should name the failed path %q, got: %v", bad, err)
	}

	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].path != good || rows[0].profile != "a good profile" {
		t.Errorf("want only %s written with its profile, got %+v", good, rows)
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
