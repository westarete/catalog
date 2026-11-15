package reader

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReadTruncates(t *testing.T) {
	line := strings.Repeat("x", 99) + "\n"
	text := strings.Repeat(line, 5000) // 500,000 bytes, 100 bytes per line

	got := Read("big.md", []byte(text))

	if !got.Truncated {
		t.Fatal("Read() Truncated = false, want true")
	}
	const cut = 399_999 // the last newline at or before 400,000 bytes
	wantNote := fmt.Sprintf("\n\n[Document truncated: showing the first %d of %d bytes. Profile it from this part.]", cut, len(text))
	if want := text[:cut] + wantNote; got.Text != want {
		t.Errorf("Read() text is not the first %d bytes plus the note", cut)
	}
	body := strings.TrimSuffix(got.Text, wantNote)
	for i, l := range strings.Split(body, "\n") {
		if len(l) != 99 {
			t.Fatalf("line %d of body has %d bytes, want 99: body cut mid-line", i, len(l))
		}
	}
}

func TestReadKeepsShortText(t *testing.T) {
	text := strings.Repeat("y", 400_000)

	got := Read("exact.txt", []byte(text))

	if got.Truncated {
		t.Error("Read() Truncated = true for text exactly at the limit, want false")
	}
	if got.Text != text {
		t.Error("Read() changed text within the limit")
	}
}

func TestReadTruncatesWithoutNewline(t *testing.T) {
	text := strings.Repeat("a", 500_000)

	got := Read("one-line.txt", []byte(text))

	if !got.Truncated {
		t.Fatal("Read() Truncated = false, want true")
	}
	if !strings.HasPrefix(got.Text, strings.Repeat("a", 400_000)+"\n\n[Document truncated") {
		t.Error("Read() without a newline did not cut at the byte limit")
	}
}

func TestReadCutsAtRuneBoundary(t *testing.T) {
	// "a" then "é" (two bytes each) puts a continuation byte at index 400,000,
	// so the cut must step back to 399,999.
	text := "a" + strings.Repeat("é", 300_000)

	got := Read("accents.txt", []byte(text))

	body, _, _ := strings.Cut(got.Text, "\n\n[Document truncated")
	if len(body) != 399_999 {
		t.Errorf("body is %d bytes, want 399999", len(body))
	}
	if !utf8.ValidString(body) {
		t.Error("body is not valid UTF-8")
	}
}

func TestReadPDF(t *testing.T) {
	data := []byte("%PDF-1.7 binary")

	got := Read("x.PDF", data)

	if !got.PDF {
		t.Error("Read() PDF = false, want true")
	}
	if string(got.Data) != string(data) {
		t.Errorf("Read() Data = %q, want input bytes", got.Data)
	}
	if got.Text != "" || got.Truncated {
		t.Errorf("Read() Text = %q, Truncated = %v, want empty and false for a PDF", got.Text, got.Truncated)
	}
}

func TestReadTextHasNoData(t *testing.T) {
	got := Read("notes.md", []byte("hello"))

	if got.PDF || got.Data != nil {
		t.Errorf("Read() PDF = %v, Data = %q, want false and nil for text", got.PDF, got.Data)
	}
	if got.Text != "hello" {
		t.Errorf("Read() Text = %q, want %q", got.Text, "hello")
	}
}

func TestListType(t *testing.T) {
	tests := []struct {
		rel  string
		want string
	}{
		{"a/b.md", "Markdown"},
		{"NOTES.MD", "Markdown"},
		{"x.pdf", "PDF"},
		{"x.txt", "text"},
		{"data.CSV", "CSV"},
		{"notes.rst", "rst"},
		{"Makefile", "file"},
		{"dir.v2/README", "file"},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			if got := List(tt.rel, []byte("x")).Type; got != tt.want {
				t.Errorf("List(%q).Type = %q, want %q", tt.rel, got, tt.want)
			}
		})
	}
}

func TestListSize(t *testing.T) {
	got := List("a.txt", []byte("hello"))

	if got.Size != 5 {
		t.Errorf("List() Size = %d, want 5", got.Size)
	}
}

func TestListFirstLines(t *testing.T) {
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	text := strings.Join(lines, "\n")

	got := List("a.txt", []byte(text))

	want := strings.Join(lines[:10], "\n")
	if got.FirstLines != want {
		t.Errorf("List() FirstLines = %q, want the first 10 lines %q", got.FirstLines, want)
	}
}

func TestListFirstLinesCapsBytes(t *testing.T) {
	text := strings.Repeat("x", 2000) + "\nsecond line"

	got := List("a.txt", []byte(text))

	if len(got.FirstLines) != 512 {
		t.Errorf("List() FirstLines is %d bytes, want 512", len(got.FirstLines))
	}
}

func TestListFirstLinesCutsAtRuneBoundary(t *testing.T) {
	// "a" then "é": index 512 is a continuation byte, so the cut steps back to 511.
	text := "a" + strings.Repeat("é", 1000)

	got := List("a.txt", []byte(text))

	if len(got.FirstLines) != 511 {
		t.Errorf("List() FirstLines is %d bytes, want 511", len(got.FirstLines))
	}
	if !utf8.ValidString(got.FirstLines) {
		t.Error("List() FirstLines is not valid UTF-8")
	}
}

func TestListInvalidUTF8(t *testing.T) {
	text := "\xff\xfe" + "hello\nworld"

	got := List("a.txt", []byte(text))

	if !utf8.ValidString(got.FirstLines) {
		t.Errorf("List() FirstLines %q is not valid UTF-8", got.FirstLines)
	}
	if !strings.Contains(got.FirstLines, "hello") {
		t.Errorf("List() FirstLines %q lost the text after the invalid bytes", got.FirstLines)
	}
}

func TestListPDFHasNoFirstLines(t *testing.T) {
	got := List("x.pdf", []byte("%PDF-1.7\nmore"))

	if got.FirstLines != "" {
		t.Errorf("List() FirstLines = %q, want empty for a PDF", got.FirstLines)
	}
	if got.Size != int64(len("%PDF-1.7\nmore")) {
		t.Errorf("List() Size = %d, want the PDF's size", got.Size)
	}
}

func TestListFirstLinesKeepsExactLimit(t *testing.T) {
	text := strings.Repeat("x", 512)

	got := List("a.txt", []byte(text))

	if got.FirstLines != text {
		t.Errorf("List() FirstLines is %d bytes, want the full 512-byte line", len(got.FirstLines))
	}
}

func TestReadCutsLongLineAfterLeadingNewline(t *testing.T) {
	// The only newline is at index 0, so there is no line boundary to cut at;
	// the cut falls back to the byte limit rather than leaving the text empty.
	text := "\n" + strings.Repeat("a", 500_000)

	got := Read("one-line.txt", []byte(text))

	want := "\n" + strings.Repeat("a", 399_999) +
		"\n\n[Document truncated: showing the first 400000 of 500001 bytes. Profile it from this part.]"
	if got.Text != want {
		t.Errorf("Read() text is %d bytes, want the first 400000 bytes plus the note", len(got.Text))
	}
}

func TestReadInvalidUTF8WithoutCharacterBoundary(t *testing.T) {
	// No byte is a character start, so the cut runs back to zero bytes.
	text := strings.Repeat("\x80", 500_000)

	got := Read("junk.txt", []byte(text))

	want := "\n\n[Document truncated: showing the first 0 of 500000 bytes. Profile it from this part.]"
	if got.Text != want {
		t.Errorf("Read() text = %q, want the note alone", got.Text)
	}
}
