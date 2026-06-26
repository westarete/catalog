package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// The profile prompt. The generator's job is not to summarize a document well —
// it is to infer the conditions under which an agent should pull the document
// into context, exactly as a skill's description tells the harness when to load
// the skill body. A wrong summary is ugly; a wrong profile makes the agent load
// the wrong context or miss the right one, which is the failure the catalog
// exists to prevent.
const profileInstruction = `You write one entry for a context catalog — a short profile that tells an AI agent when to open a document. Think of it like the "description" on an agent skill: plain language that helps the agent decide whether to load this file, not a keyword list and not a summary.

You get the full text of one document, plus the existing profiles for the other documents so you can tell this one apart from its neighbors.

Write two short sentences:
1. Say what the document is, in a few words.
2. Say when to open it — the task or question that should send the agent here.

Cross-reference sparingly. Add a "see X.md" pointer only when another document is a near-duplicate the agent could easily confuse with this one — two files that cover the same ground, where the agent needs to know which to pick. Do not point at every neighbor that touches an adjacent topic; a dense web of "see also" links is noise, not help. Most entries need no cross-reference at all. When you do add one, say which to use when, and name the file.

Write the way you'd tell a colleague which file to grab. Lead with the subject or a plain verb; put the point first. Keep sentences short.

Hard rules on sentence shape:
- Never open with a long "When the task involves … — this document should be loaded" windup. Burying the verb behind a 30-word clause is the one structure to avoid.
- No filler verb phrases like "this document should be loaded" or "consult this when." Just say what it's for.

Good (no near-duplicate, so no pointer): "West Arete's service tiers — what each costs, what it delivers, who must be in the room. Open it for the mechanics of the offering."

Good (a real near-duplicate to disambiguate): "Prospect-qualifying reference: org size, buying coalition, trigger events, disqualifiers. Open it to judge whether a specific org is a fit. A near-identical file lives at gtm_strategy/02_ideal_customer_profile.md — for active GTM work, use that one; this is the source it was built from."

Bad: "When the task involves understanding the full structure, pricing, and scope of West Arete's service tiers — for instance before drafting a proposal — this document should be loaded."

Speak plainly; this file is published and follows the same voice as the rest of the repo. Don't call a document "canonical," "authoritative," or "definitive" unless it says so itself. Describe only what the document's own text supports. Return only the profile — no heading, no path, no preamble.`

// cmdBootstrap rebuilds every entry from scratch in two passes. Pass one has no
// neighbor profiles to read, so it writes provisional entries; pass two re-infers
// each with every neighbor's real profile in view. This is the only command that
// needs two passes — and the only reason it does is that pass one started from an
// empty slate. Use it when creating .catalog.md or rebuilding it wholesale.
func cmdBootstrap(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("bootstrap: takes no arguments")
	}
	docs, cat, cfg, err := loadForGenerate()
	if err != nil {
		return err
	}
	pruneOrphans(cat.entries, docs)
	return runGenerate(cfg, cat, docs, 2)
}

// cmdUpdate re-infers profiles only for the documents Git reports changed (or
// missing an entry) and rewrites those in place, leaving the rest of .catalog.md
// untouched. One pass: the changed documents read their untouched neighbors'
// real profiles. This is the routine job, like bin/format — run it after editing
// a document.
func cmdUpdate(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("update: takes no arguments")
	}
	docs, cat, cfg, err := loadForGenerate()
	if err != nil {
		return err
	}
	if err := requirePopulated(cat); err != nil {
		return err
	}
	ref, err := lastTouchedCommit(execGit, catalogPath)
	if err != nil {
		return err
	}
	var stale map[string]bool
	if stale, err = staleDocs(execGit, ref, docs); err != nil {
		return err
	}
	targets := selectStale(docs, cat.entries, stale)
	pruneOrphans(cat.entries, docs)

	if len(targets) == 0 {
		fmt.Printf("catalog: up to date (%d docs).\n", len(docs))
		return writeCatalog(cat) // normalize layout even when no profiles change
	}
	return runGenerate(cfg, cat, targets, 1)
}

// cmdForce re-infers the named documents even when Git thinks they're current,
// in one pass — each reads its neighbors' existing profiles. With no arguments it
// forces every document. Use it to redo a few entries you're unhappy with, or to
// rebuild all entries in one pass after a prompt change (cheaper than bootstrap,
// which a populated catalog doesn't need).
func cmdForce(args []string) error {
	docs, cat, cfg, err := loadForGenerate()
	if err != nil {
		return err
	}
	if err := requirePopulated(cat); err != nil {
		return err
	}
	targets := docs
	if len(args) > 0 {
		if targets, err = resolveForceTargets(args, docs); err != nil {
			return err
		}
	}
	pruneOrphans(cat.entries, docs)
	return runGenerate(cfg, cat, targets, 1)
}

// loadForGenerate reads the three inputs every key-using command needs: the
// enumerated documents, the current catalog, and the config.
func loadForGenerate() (docs []string, cat *catalog, cfg *config, err error) {
	cfg, err = loadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	docs, err = includedDocs(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(docs) == 0 {
		return nil, nil, nil, fmt.Errorf("no documents enumerated — check %s", configPath)
	}
	cat, err = readCatalog()
	if err != nil {
		return nil, nil, nil, err
	}
	return docs, cat, cfg, nil
}

// requirePopulated refuses to run update or force against an empty or missing
// catalog. Both write a single pass, which assumes the entries they leave alone
// already hold real profiles for documents to read as neighbors. With nothing
// there, a one-pass build produces a catalog where every entry was written
// blind — exactly the case bootstrap's second pass exists to fix. Point the user
// at it instead of silently doing the worse thing.
func requirePopulated(cat *catalog) error {
	if len(cat.entries) == 0 {
		return fmt.Errorf("%s has no entries — run `bin/catalog bootstrap` to build it", catalogPath)
	}
	return nil
}

// resolveForceTargets validates the filenames a user passed to force, returning
// them in the canonical docs order. A name that isn't an enumerated document is
// an error naming the config, not a silent no-op — typos shouldn't burn an API
// call on an untracked path or quietly do nothing.
func resolveForceTargets(args, docs []string) ([]string, error) {
	enumerated := map[string]bool{}
	for _, d := range docs {
		enumerated[d] = true
	}
	want := map[string]bool{}
	for _, a := range args {
		if !enumerated[a] {
			return nil, fmt.Errorf("%q is not an enumerated document — check %s", a, configPath)
		}
		want[a] = true
	}
	var targets []string
	for _, d := range docs {
		if want[d] {
			targets = append(targets, d)
		}
	}
	return targets, nil
}

// runGenerate is the shared body behind bootstrap, update, and force: resolve the
// key, infer profiles for the targets in the given number of passes, write the
// catalog, and print the run summary. The caller has already chosen the targets
// and the pass count — the two knobs that distinguish the three commands.
func runGenerate(cfg *config, cat *catalog, targets []string, passes int) error {
	key, err := loadAPIKey(cfg)
	if err != nil {
		return err
	}
	client := anthropic.NewClient(option.WithAPIKey(key))
	ctx := context.Background()
	start := time.Now()
	var total tokens

	for p := 1; p <= passes; p++ {
		if passes > 1 {
			fmt.Fprintf(os.Stderr, "  pass %d of %d...\n", p, passes)
		}
		used, err := inferPass(ctx, &client, cfg, cat, targets)
		total = total.add(used)
		if err != nil {
			return err
		}
	}

	if err := writeCatalog(cat); err != nil {
		return err
	}
	fmt.Println(formatRunSummary(len(targets), passes, total, time.Since(start)))
	return nil
}

// formatRunSummary renders the end-of-run report: documents, passes, the API
// call count, the token counts the run consumed, and wall-clock. No dollar
// figure — multiply the token counts by the current published rate for that.
func formatRunSummary(docs, passes int, t tokens, elapsed time.Duration) string {
	return fmt.Sprintf(
		"catalog: %d docs in %d pass(es) — %d calls, %d input + %d cached + %d output tokens, %.0fs",
		docs, passes, docs*passes, t.input, t.cached, t.output, elapsed.Seconds())
}

// selectStale picks the documents update should re-infer: those Git reports
// stale, plus any document missing an entry. Targets follow the input order of
// docs (which callers keep sorted) so a run is deterministic.
func selectStale(docs []string, entries map[string]*entry, stale map[string]bool) []string {
	var targets []string
	for _, d := range docs {
		if stale[d] || entries[d] == nil {
			targets = append(targets, d)
		}
	}
	return targets
}

// pruneOrphans removes entries whose document is no longer enumerated (deleted
// or now ignored), mutating entries in place.
func pruneOrphans(entries map[string]*entry, docs []string) {
	present := map[string]bool{}
	for _, d := range docs {
		present[d] = true
	}
	for p := range entries {
		if !present[p] {
			delete(entries, p)
		}
	}
}

// inferPass infers a fresh profile for each target document and writes it into
// the catalog, using every other entry as neighbor context. It returns the
// token counts the pass consumed.
func inferPass(ctx context.Context, client *anthropic.Client, cfg *config, cat *catalog, targets []string) (tokens, error) {
	var total tokens
	for _, path := range targets {
		text, err := os.ReadFile(path)
		if err != nil {
			return total, fmt.Errorf("reading %s: %w", path, err)
		}
		fmt.Fprintf(os.Stderr, "  profile: %s\n", path)
		profile, used, err := inferProfile(ctx, client, cfg, path, string(text), neighbors(cat, path))
		if err != nil {
			return total, fmt.Errorf("%s: %w", path, err)
		}
		cat.entries[path] = &entry{path: path, profile: profile}
		total = total.add(used)
	}
	return total, nil
}

// neighbors renders the other entries' profiles as context for positioning.
func neighbors(cat *catalog, self string) string {
	paths := make([]string, 0, len(cat.entries))
	for p := range cat.entries {
		if p != self {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "- %s: %s\n", p, cat.entries[p].profile)
	}
	return b.String()
}

// buildProfilePrompt assembles the user message: the document under a path-
// labeled tag, followed by the existing neighbor profiles (if any) as
// positioning context. Pure string work, separated from the API call so the
// prompt shape can be tested without a key.
func buildProfilePrompt(path, text, neighborText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<document path=%q>\n%s\n</document>\n", path, text)
	if neighborText != "" {
		b.WriteString("\n# Existing profiles for other documents in this catalog\n\n")
		b.WriteString(neighborText)
	}
	return b.String()
}

// tokens holds the token counts the API reports for the work a run did. They
// are the durable facts a run produces; converting them to a dollar cost needs
// a current price, which the program deliberately does not carry.
type tokens struct {
	input, cached, output int64
}

func (t tokens) add(o tokens) tokens {
	return tokens{t.input + o.input, t.cached + o.cached, t.output + o.output}
}

func inferProfile(ctx context.Context, client *anthropic.Client, cfg *config, path, text, neighborText string) (string, tokens, error) {
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(cfg.Model),
		MaxTokens: 400,
		System:    []anthropic.TextBlockParam{{Text: profileInstruction}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildProfilePrompt(path, text, neighborText))),
		},
	})
	if err != nil {
		return "", tokens{}, err
	}
	var out strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			out.WriteString(block.Text)
		}
	}
	t := strings.TrimSpace(out.String())
	if t == "" {
		return "", tokens{}, fmt.Errorf("model returned no profile text")
	}
	used := tokens{
		input:  resp.Usage.InputTokens,
		cached: resp.Usage.CacheReadInputTokens,
		output: resp.Usage.OutputTokens,
	}
	return t, used, nil
}
