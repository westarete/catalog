// Package reader turns a file's bytes into what the model reads (Content) and
// into the cheap listing shown before a summary exists (Listing). It does no
// I/O: the caller passes the file's path relative to the watched folder and
// its bytes.
package reader

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// Version identifies the reader's output rules. It is part of the recipe, so
// changing how text is read or listed changes the recipe version.
const Version = "1"

// maxTextBytes caps how much of a text document is sent to the model. The API
// rejects a prompt over 1,000,000 tokens, and a large CSV can run past that
// (one 1.5M-token file failed every run). A profile only needs enough of a
// document to tell what it is and when to open it, so the rest is cut. 400,000
// bytes stays well under the limit even for dense, token-hungry data like CSV,
// and is far beyond any prose document the catalog is meant for.
const maxTextBytes = 400_000

// listingLines and listingBytes bound the first lines shown in a Listing.
const (
	listingLines = 10
	listingBytes = 512
)

// Content is what the model reads for one file. A PDF goes in whole as Data;
// any other file goes in as Text, cut to maxTextBytes when it is longer.
type Content struct {
	PDF       bool
	Text      string
	Data      []byte
	Truncated bool
}

// Listing is the cheap description of a file shown before a summary exists.
type Listing struct {
	Type       string
	Size       int64
	FirstLines string
}

// Read returns the content the model reads for the file at rel.
func Read(rel string, data []byte) Content {
	if isPDF(rel) {
		return Content{PDF: true, Data: data}
	}
	text := string(data)
	return Content{
		Text:      truncateDocument(text, maxTextBytes),
		Truncated: len(text) > maxTextBytes,
	}
}

// List describes the file at rel without reading it for the model.
func List(rel string, data []byte) Listing {
	listing := Listing{Type: fileType(rel), Size: int64(len(data))}
	if !isPDF(rel) {
		listing.FirstLines = firstLines(string(data))
	}
	return listing
}

// isPDF reports whether rel has a .pdf extension, ignoring case.
func isPDF(rel string) bool {
	return strings.EqualFold(path.Ext(rel), ".pdf")
}

// typeByExt maps lowercase extensions to the names shown in a listing.
var typeByExt = map[string]string{
	".md":  "Markdown",
	".pdf": "PDF",
	".txt": "text",
	".csv": "CSV",
}

// fileType names the file's kind: a known name from typeByExt, otherwise the
// extension without its dot, or "file" when there is no extension.
func fileType(rel string) string {
	ext := path.Ext(rel)
	if name, ok := typeByExt[strings.ToLower(ext)]; ok {
		return name
	}
	if name := strings.TrimPrefix(ext, "."); name != "" {
		return name
	}
	return "file"
}

// firstLines returns the first listingLines lines of text, at most
// listingBytes bytes. Invalid UTF-8 is replaced first, so the cut below always
// lands on a valid boundary and the result is valid UTF-8.
func firstLines(text string) string {
	// SplitN returns one element more than listingLines only when the text has
	// more lines than that; the extra element is the rest of the text, so drop it.
	lines := strings.SplitN(text, "\n", listingLines+1)
	if len(lines) == listingLines+1 {
		lines = lines[:listingLines]
	}
	valid := strings.ToValidUTF8(strings.Join(lines, "\n"), "�")
	return cutAtRuneBoundary(valid, listingBytes)
}

// cutAtRuneBoundary returns s cut to at most limit bytes without splitting a
// character. s must be valid UTF-8.
func cutAtRuneBoundary(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// truncateDocument returns text cut to at most limit bytes, ending at a line
// boundary (or, if one line is longer than the limit, at a character boundary)
// with a note telling the model the rest was left out. Text within the limit
// comes back unchanged.
func truncateDocument(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	if i := strings.LastIndexByte(text[:cut], '\n'); i > 0 {
		cut = i
	} else {
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
	}
	return fmt.Sprintf("%s\n\n[Document truncated: showing the first %d of %d bytes. Profile it from this part.]", text[:cut], cut, len(text))
}
