package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// profileMaxTokens caps the response to one profile request. A profile is only a
// few hundred tokens, but models that think by default (Sonnet 5 and later)
// spend output tokens thinking before they write, and a document that is hard
// to position against its neighbors can use up a small cap before any profile
// text appears. A near-duplicate transcript in one repo failed at 1024 and 2048
// and succeeded using 848 to 1856 tokens at higher caps; usage varies from run
// to run. The cap bounds what one runaway document can cost, so it is set with
// headroom over that, not as high as the API allows.
const profileMaxTokens = 4096

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

// cmdBootstrap rebuilds every entry from scratch in two passes, ignoring any
// existing store or .catalog.md entirely — the database is the sole source
// of truth, so a repo with no database always gets a full rebuild rather
// than a partial-trust import of whatever .catalog.md happens to say (see
// FUTURE.md). Pass one has no neighbor profiles to read, so it writes
// provisional entries; pass two re-infers each with every neighbor's real
// profile in view. This is the only command that needs two passes — and the
// only reason it does is that pass one started from an empty slate.
func cmdBootstrap(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("bootstrap: takes no arguments")
	}
	docs, cfg, err := loadDocsAndConfig()
	if err != nil {
		return err
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Bootstrap starts from nothing: drop every existing row so pass one
	// truly has no neighbor context, matching a fresh database.
	if _, err := db.Exec(`DELETE FROM profiles`); err != nil {
		return fmt.Errorf("clearing store: %w", err)
	}
	return runGenerate(db, cfg, docs, 2)
}

// cmdUpdate re-infers profiles only for the documents that are new or
// modified relative to the store, drops rows for documents that are deleted
// or no longer enumerated, and always rewrites .catalog.md — even when no
// profile changes, so the file stays a faithful render of the store. This is
// the routine job, like bin/format — run it after editing a document.
func cmdUpdate(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("update: takes no arguments")
	}
	docs, cfg, err := loadDocsAndConfig()
	if err != nil {
		return err
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	if err := requirePopulated(rows); err != nil {
		return err
	}

	docHashes, err := hashDocs(docs)
	if err != nil {
		return err
	}
	status := classify(docHashes, rows)
	for _, path := range status.deleted {
		if err := deleteProfile(db, path); err != nil {
			return err
		}
	}
	if report := formatDeletedReport(status.deleted); report != "" {
		fmt.Println(report)
	}
	targets := append(append([]string{}, status.new...), status.modified...)
	sort.Strings(targets)

	if len(targets) == 0 {
		if len(status.deleted) == 0 {
			fmt.Printf("catalog: up to date (%d docs).\n", len(docs))
		}
		return regenerateCatalogMD(db) // normalize layout even when no profiles change
	}
	return runGenerate(db, cfg, targets, 1)
}

// formatDeletedReport lists the documents whose profiles update dropped, one
// per line under a count, or returns "" when nothing was dropped.
func formatDeletedReport(deleted []string) string {
	if len(deleted) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "catalog: removed %d deleted doc(s):", len(deleted))
	for _, path := range deleted {
		fmt.Fprintf(&b, "\n  %s", path)
	}
	return b.String()
}

// cmdForce re-infers the named documents (or all, with no names)
// unconditionally, in one pass — each reads its neighbors' existing
// profiles. Use it to redo a few entries you're unhappy with, or to rebuild
// all entries in one pass after a prompt change (cheaper than bootstrap,
// which a populated store doesn't need).
func cmdForce(args []string) error {
	docs, cfg, err := loadDocsAndConfig()
	if err != nil {
		return err
	}
	db, err := openStore(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	if err := requirePopulated(rows); err != nil {
		return err
	}

	targets := docs
	if len(args) > 0 {
		if targets, err = resolveForceTargets(args, docs); err != nil {
			return err
		}
	}

	docHashes, err := hashDocs(docs)
	if err != nil {
		return err
	}
	for _, path := range classify(docHashes, rows).deleted {
		if err := deleteProfile(db, path); err != nil {
			return err
		}
	}
	return runGenerate(db, cfg, targets, 1)
}

// loadDocsAndConfig reads the two inputs every command needs before it can
// touch the store: the enumerated documents and the config.
func loadDocsAndConfig() (docs []string, cfg *config, err error) {
	cfg, err = loadConfig()
	if err != nil {
		return nil, nil, err
	}
	docs, err = includedDocs(cfg)
	if err != nil {
		return nil, nil, err
	}
	if len(docs) == 0 {
		return nil, nil, fmt.Errorf("no documents enumerated — check %s", configPath)
	}
	return docs, cfg, nil
}

// hashDocs reads every document in docs and returns its current content
// hash, keyed by path.
func hashDocs(docs []string) (map[string]string, error) {
	hashes := make(map[string]string, len(docs))
	for _, path := range docs {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		hashes[path] = contentHash(content)
	}
	return hashes, nil
}

// requirePopulated refuses to run update or force against an empty store.
// Both write a single pass, which assumes the rows they leave alone already
// hold real profiles for documents to read as neighbors. With nothing there,
// a one-pass build produces entries written blind — exactly the case
// bootstrap's second pass exists to fix. Point the user at it instead of
// silently doing the worse thing.
func requirePopulated(rows []profileRow) error {
	if len(rows) == 0 {
		return fmt.Errorf("%s has no entries — run `catalog bootstrap` to build it", storePath)
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

// runGenerate is the shared body behind bootstrap, update, and force: resolve
// the key, infer profiles for the targets in the given number of passes,
// writing each one into the store as it's produced so later targets in the
// same pass (and every target in a later pass) read it as a real neighbor,
// then regenerate .catalog.md and print the run summary. The caller has
// already chosen the targets and the pass count — the two knobs that
// distinguish the three commands.
func runGenerate(db *sql.DB, cfg *config, targets []string, passes int) error {
	key, err := loadAPIKey(cfg)
	if err != nil {
		return err
	}
	// WithoutEnvironmentDefaults: catalog already resolved the key itself via
	// loadAPIKey (env var, then the profile named in .catalog/config.toml,
	// then CATALOG_PROFILE, then [default] in the credentials file). Without
	// this, NewClient also auto-loads the SDK's own separate profile config
	// file if one exists on the machine, which triggers a harmless but
	// confusing "ANTHROPIC_API_KEY is set and takes precedence" warning even
	// when no such env var is set — the SDK found its own config file, not
	// anything catalog did.
	client := anthropic.NewClient(option.WithoutEnvironmentDefaults(), option.WithAPIKey(key))
	ctx := context.Background()
	start := time.Now()
	var total tokens

	var passErr error
	for p := 1; p <= passes; p++ {
		if passes > 1 {
			fmt.Fprintf(os.Stderr, "  pass %d of %d...\n", p, passes)
		}
		used, err := inferPass(ctx, db, &client, cfg, targets)
		total = total.add(used)
		if err != nil {
			passErr = err
			break
		}
	}

	// Regenerate .catalog.md from whatever landed in the store even on a
	// partial failure — the successful profiles from this run are already
	// written, and the rendered file should never lag behind the database.
	if err := regenerateCatalogMD(db); err != nil {
		return err
	}
	if passErr != nil {
		return passErr
	}
	fmt.Println(formatRunSummary(len(targets), passes, total, time.Since(start)))
	return nil
}

// regenerateCatalogMD renders .catalog.md fresh from every row in the store
// and writes it to disk — the database is the sole source of truth, so this
// runs after every write, even one that changed nothing, to normalize
// layout and correct any drift.
func regenerateCatalogMD(db *sql.DB) error {
	rows, err := readProfiles(db)
	if err != nil {
		return err
	}
	return os.WriteFile(catalogPath, []byte(render(rows)), 0o644)
}

// formatRunSummary renders the end-of-run report: documents, passes, the API
// call count, the token counts the run consumed, and wall-clock. No dollar
// figure — multiply the token counts by the current published rate for that.
func formatRunSummary(docs, passes int, t tokens, elapsed time.Duration) string {
	return fmt.Sprintf(
		"catalog: %d docs in %d pass(es) — %d calls, %d input + %d cached + %d output tokens, %.0fs",
		docs, passes, docs*passes, t.input, t.cached, t.output, elapsed.Seconds())
}

// inferPass infers a fresh profile for each target document and writes it
// into the store immediately, using every other row as neighbor context. It
// returns the token counts the pass consumed. A document whose inference
// fails (e.g. the model hit its token cap before producing profile text) is
// reported to stderr and skipped rather than aborting the whole pass — one
// bad document in a batch of hundreds shouldn't block every other entry from
// being written.
func inferPass(ctx context.Context, db *sql.DB, client *anthropic.Client, cfg *config, targets []string) (tokens, error) {
	var total tokens
	var failed []string
	for _, path := range targets {
		text, err := os.ReadFile(path)
		if err != nil {
			return total, fmt.Errorf("reading %s: %w", path, err)
		}
		neighborText, err := neighbors(db, path)
		if err != nil {
			return total, err
		}
		fmt.Fprintf(os.Stderr, "  %s\n", path)
		profile, used, err := inferProfile(ctx, client, cfg, path, text, neighborText)
		total = total.add(used)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  catalog: %s: %v\n", path, err)
			failed = append(failed, path)
			continue
		}
		row := profileRow{path: path, contentHash: contentHash(text), profile: profile}
		if err := writeProfile(db, row); err != nil {
			return total, err
		}
	}
	if len(failed) > 0 {
		return total, fmt.Errorf("%d of %d documents failed: %s", len(failed), len(targets), strings.Join(failed, ", "))
	}
	return total, nil
}

// neighbors renders every other row's profile as context for positioning.
func neighbors(db *sql.DB, self string) (string, error) {
	rows, err := readProfiles(db)
	if err != nil {
		return "", err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	var b strings.Builder
	for _, r := range rows {
		if r.path != self {
			fmt.Fprintf(&b, "- %s: %s\n", r.path, r.profile)
		}
	}
	return b.String(), nil
}

// buildProfilePrompt assembles the user message: the document under a path-
// labeled tag, followed by the existing neighbor profiles (if any) as
// positioning context. Pure string work, separated from the API call so the
// prompt shape can be tested without a key.
func buildProfilePrompt(path, text, neighborText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<document path=%q>\n%s\n</document>\n", path, text)
	b.WriteString(buildNeighborPrompt(neighborText))
	return b.String()
}

// buildNeighborPrompt renders the "existing profiles" positioning section on
// its own, for callers (like the PDF path) that send the document itself as
// a separate content block rather than inlined in the text prompt.
func buildNeighborPrompt(neighborText string) string {
	if neighborText == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n# Existing profiles for other documents in this catalog\n\n")
	b.WriteString(neighborText)
	return b.String()
}

// isPDF reports whether path names a PDF document, which the model reads
// natively as a document content block instead of inlined text.
func isPDF(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".pdf")
}

// maxDocumentBytes caps how much of a text document is sent to the model. The
// API rejects a prompt over 1,000,000 tokens, and a large CSV can run past that
// (one 1.5M-token file failed every run). A profile only needs enough of a
// document to tell what it is and when to open it, so the rest is cut. 400,000
// bytes stays well under the limit even for dense, token-hungry data like CSV,
// and is far beyond any prose document the catalog is meant for.
const maxDocumentBytes = 400_000

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

// buildDocumentBlocks assembles the content blocks for the user message: a
// PDF is sent as a native document block (plus a separate text block for
// neighbor positioning, since that context isn't part of the file itself);
// anything else is sent as a single inlined text block, as before.
func buildDocumentBlocks(path string, content []byte, neighborText string) []anthropic.ContentBlockParamUnion {
	if !isPDF(path) {
		return []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock(buildProfilePrompt(path, truncateDocument(string(content), maxDocumentBytes), neighborText)),
		}
	}
	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{
			Data: base64.StdEncoding.EncodeToString(content),
		}),
	}
	if neighbors := buildNeighborPrompt(neighborText); neighbors != "" {
		blocks = append(blocks, anthropic.NewTextBlock(neighbors))
	}
	return blocks
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

func inferProfile(ctx context.Context, client *anthropic.Client, cfg *config, path string, content []byte, neighborText string) (string, tokens, error) {
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(cfg.Model),
		MaxTokens: profileMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: profileInstruction}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(buildDocumentBlocks(path, content, neighborText)...),
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
		if resp.StopReason == anthropic.StopReasonRefusal {
			return "", tokens{}, fmt.Errorf("model refused (%s): %s", resp.StopDetails.Category, resp.StopDetails.Explanation)
		}
		return "", tokens{}, fmt.Errorf("model returned no profile text (stop_reason=%s)", resp.StopReason)
	}
	used := tokens{
		input:  resp.Usage.InputTokens,
		cached: resp.Usage.CacheReadInputTokens,
		output: resp.Usage.OutputTokens,
	}
	return t, used, nil
}
