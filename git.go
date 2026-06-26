package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// staleness is computed entirely from Git — no stored hashes. Git is already a
// content-addressed store that knows what changed between any two states; the
// reference point is the commit that last touched .catalog.md, and a document
// is stale if it changed after that commit or has uncommitted working-tree
// edits. See the "Staleness is a Git query" section of SKILL.md for the full
// rationale.
//
// The I/O (running git) and the logic (deciding staleness from git's output)
// are kept separate: gitRunner does the calls, computeStale and
// parsePorcelainPaths are pure functions over their string output, so the
// decision logic is tested without a repo.

// gitRunner runs git and returns trimmed stdout. The function type lets tests
// substitute canned output for real git calls.
type gitRunner func(args ...string) (string, error)

func execGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s",
				strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// lastTouchedCommit returns the SHA of the most recent commit that modified
// path, or "" if git has no commit touching it (e.g. .catalog.md has never been
// committed — first generation).
func lastTouchedCommit(run gitRunner, path string) (string, error) {
	return run("log", "-1", "--format=%H", "--", path)
}

// staleDocs runs the two git queries and hands their output to computeStale.
func staleDocs(run gitRunner, ref string, docs []string) (map[string]bool, error) {
	if ref == "" {
		return allStale(docs), nil
	}
	committed, err := run("diff", "--name-only", ref, "HEAD")
	if err != nil {
		return nil, err
	}
	working, err := run("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return computeStale(ref, docs, committed, working), nil
}

// computeStale is the pure decision: given the reference commit, the documents
// we care about, the names from `git diff --name-only ref HEAD`, and the lines
// from `git status --porcelain`, return which documents are stale. An empty ref
// means no prior catalog commit, so everything is stale.
func computeStale(ref string, docs []string, committedDiff, porcelainStatus string) map[string]bool {
	if ref == "" {
		return allStale(docs)
	}
	want := map[string]bool{}
	for _, d := range docs {
		want[d] = true
	}
	stale := map[string]bool{}
	for p := range strings.SplitSeq(committedDiff, "\n") {
		if p = strings.TrimSpace(p); want[p] {
			stale[p] = true
		}
	}
	for _, p := range parsePorcelainPaths(porcelainStatus) {
		if want[p] {
			stale[p] = true
		}
	}
	return stale
}

// parsePorcelainPaths extracts the affected path from each line of
// `git status --porcelain` output: two status columns, a space, then the path;
// a rename renders as "old -> new", from which we take the destination.
func parsePorcelainPaths(status string) []string {
	var paths []string
	for line := range strings.SplitSeq(status, "\n") {
		if len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[2:])
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		paths = append(paths, p)
	}
	return paths
}

func allStale(docs []string) map[string]bool {
	stale := map[string]bool{}
	for _, d := range docs {
		stale[d] = true
	}
	return stale
}
