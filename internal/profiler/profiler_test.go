package profiler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/reader"
)

type fakeProvider struct {
	got  models.Request
	resp models.Response
	err  error
}

func (f *fakeProvider) Complete(_ context.Context, req models.Request) (models.Response, error) {
	f.got = req
	return f.resp, f.err
}

var neighbors = []Neighbor{
	{Rel: "z.md", Summary: "Last one."},
	{Rel: "notes.md", Summary: "Its own old summary."},
	{Rel: "a.md", Summary: "First one."},
}

func TestProfileTextRequest(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: " A profile. \n", InputTokens: 7, OutputTokens: 3}}
	text, usage, err := Profile(context.Background(), f, "notes.md", reader.Content{Text: "hello"}, neighbors)
	if err != nil {
		t.Fatal(err)
	}
	if text != "A profile." {
		t.Errorf("text = %q", text)
	}
	if usage.InputTokens != 7 || usage.OutputTokens != 3 {
		t.Errorf("usage = %+v", usage)
	}
	if f.got.System != profileInstruction {
		t.Error("System is not profileInstruction")
	}
	if f.got.MaxTokens != maxTokens || maxTokens != 4096 {
		t.Errorf("MaxTokens = %d", f.got.MaxTokens)
	}
	if f.got.PDF != nil {
		t.Error("PDF set for text content")
	}
	want := "<document path=\"notes.md\">\nhello\n</document>\n" +
		"\n# Existing profiles for other documents in this catalog\n\n" +
		"- a.md: First one.\n- z.md: Last one.\n"
	if f.got.Text != want {
		t.Errorf("Text = %q\nwant   %q", f.got.Text, want)
	}
}

func TestProfileTextWithoutNeighbors(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: "ok"}}
	_, _, err := Profile(context.Background(), f, "notes.md", reader.Content{Text: "hello"}, []Neighbor{{Rel: "notes.md", Summary: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<document path=\"notes.md\">\nhello\n</document>\n"; f.got.Text != want {
		t.Errorf("Text = %q, want %q", f.got.Text, want)
	}
}

func TestProfilePDFRequest(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: "ok"}}
	pdf := []byte("%PDF-1.7")
	_, _, err := Profile(context.Background(), f, "a.pdf", reader.Content{PDF: true, Data: pdf}, neighbors)
	if err != nil {
		t.Fatal(err)
	}
	if string(f.got.PDF) != string(pdf) {
		t.Errorf("PDF = %q", f.got.PDF)
	}
	want := "\n# Existing profiles for other documents in this catalog\n\n" +
		"- a.md: First one.\n- notes.md: Its own old summary.\n- z.md: Last one.\n"
	if f.got.Text != want {
		t.Errorf("Text = %q, want %q", f.got.Text, want)
	}
	if strings.Contains(f.got.Text, "<document") {
		t.Error("PDF text holds a document tag")
	}
}

func TestProfilePDFWithoutNeighbors(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: "ok"}}
	_, _, err := Profile(context.Background(), f, "a.pdf", reader.Content{PDF: true, Data: []byte("x")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.got.Text != "" {
		t.Errorf("Text = %q, want empty", f.got.Text)
	}
}

func TestProfileDoesNotReorderCallersSlice(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: "ok"}}
	in := []Neighbor{{Rel: "z.md"}, {Rel: "a.md"}}
	_, _, _ = Profile(context.Background(), f, "x.md", reader.Content{Text: "t"}, in)
	if in[0].Rel != "z.md" {
		t.Error("Profile sorted the caller's slice")
	}
}

func TestProfileEmptyReply(t *testing.T) {
	f := &fakeProvider{resp: models.Response{Text: "  \n"}}
	_, _, err := Profile(context.Background(), f, "notes.md", reader.Content{Text: "hello"}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if models.Classify(err) != models.FileProblem {
		t.Errorf("Classify = %v", models.Classify(err))
	}
}

func TestProfileProviderError(t *testing.T) {
	boom := errors.New("dial tcp")
	f := &fakeProvider{err: boom}
	_, _, err := Profile(context.Background(), f, "notes.md", reader.Content{Text: "hello"}, nil)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	if models.Classify(err) != models.ServiceProblem {
		t.Error("want ServiceProblem")
	}
}

func TestRecipe(t *testing.T) {
	if got, want := Recipe("claude-haiku-5-5"), "reader=1;prompt=1;model=claude-haiku-5-5"; string(got) != want {
		t.Errorf("Recipe = %q, want %q", got, want)
	}
}
