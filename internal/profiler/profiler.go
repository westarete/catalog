// Package profiler asks a model to write one catalog entry: a short profile
// that tells an agent when to open a document.
package profiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"westarete.com/catalog/internal/model"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/reader"
)

// PromptVersion identifies the prompt and how the request is built. It is part
// of the recipe, so changing either changes the recipe version.
const PromptVersion = "1"

// maxTokens caps the response to one profile request. A profile is only a few
// hundred tokens, but models that think by default (Sonnet 5 and later) spend
// output tokens thinking before they write, and a document that is hard to
// position against its neighbors can use up a small cap before any profile
// text appears. A near-duplicate transcript in one repo failed at 1024 and 2048
// and succeeded using 848 to 1856 tokens at higher caps; usage varies from run
// to run. The cap bounds what one runaway document can cost, so it is set with
// headroom over that, not as high as the API allows.
const maxTokens = 4096

// Neighbor is another document's existing profile, shown so the model can tell
// the document apart from it.
type Neighbor struct{ Rel, Summary string }

// Recipe names everything that shapes a profile besides the file's bytes.
func Recipe(name string) model.Recipe {
	return model.Recipe("reader=" + reader.Version + ";prompt=" + PromptVersion + ";model=" + name)
}

// Profile asks the provider for the profile of the document at rel. The
// neighbors are sorted by path, and rel's own entry is left out.
func Profile(ctx context.Context, p models.Provider, rel string, c reader.Content, neighbors []Neighbor) (string, models.Response, error) {
	req := models.Request{
		System:    profileInstruction,
		MaxTokens: maxTokens,
	}
	section := neighborSection(rel, neighbors)
	if c.PDF {
		req.PDF = c.Data
		req.Text = section
	} else {
		req.Text = fmt.Sprintf("<document path=%q>\n%s\n</document>\n", rel, c.Text) + section
	}
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return "", models.Response{}, err
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return "", resp, models.ErrEmptyReply
	}
	return text, resp, nil
}

// neighborSection renders the existing profiles of the other documents as
// positioning context, or "" when there are none.
func neighborSection(rel string, neighbors []Neighbor) string {
	others := slices.DeleteFunc(slices.Clone(neighbors), func(n Neighbor) bool { return n.Rel == rel })
	if len(others) == 0 {
		return ""
	}
	slices.SortFunc(others, func(a, b Neighbor) int { return strings.Compare(a.Rel, b.Rel) })
	var b strings.Builder
	b.WriteString("\n# Existing profiles for other documents in this catalog\n\n")
	for _, n := range others {
		fmt.Fprintf(&b, "- %s: %s\n", n.Rel, n.Summary)
	}
	return b.String()
}
