# Tools for building the catalog with autonomous coding agents

We want agents to build the catalog daemon with as little supervision as
possible, and with tests good enough to trust. This brief covers the
harnesses, plugins, and practices we looked at in November 2026 for that
purpose. We decided to build with Claude Code and the Superpowers
plugin; the last section says why.

## Harnesses

A harness is the program that runs the model, gives it tools, and
manages its context: Claude Code, OpenAI's Codex CLI, OpenCode, and
others. Graham Miranda's comparison of 28 harnesses, with data from
October 22, 2026, is the most thorough survey we found
([Miranda, October 2026](https://tech.grahammiranda.com/ai-coding-harness-comparison/)).

- The harness matters less than it used to. Miranda reports that "Claude
  Opus 5.5 resolves 64.8% of tasks inside Claude Code and 65.2% inside
  mini-SWE-agent," a harness of about 100 lines, and concludes that "for
  the strongest models, a 100-line harness gets nearly the same pass
  rate as a lab's flagship agent."
- Miranda calls Claude Code's extension model "the deepest extension
  model in the field," and also says "it's the heaviest harness to run"
  and "the sandbox is off by default."
- Codex has "a sandbox that's on by default," and in Miranda's figures
  it costs far less per task than Claude Code at maximum effort.

Our reasoning, not Miranda's: the quality we want comes from the process
around the model more than from the harness, and Claude Code's hooks,
skills, and subagents are what we'd build that process from. Codex may
still be useful as a reviewer, since a model from another family is
likely to miss different things, but we haven't tested that.

## Superpowers

Superpowers is a set of skills by Jesse Vincent that walks an agent from
brainstorming through planning, test-driven implementation, and review
([obra/superpowers](https://github.com/obra/superpowers)). It installs
from Anthropic's official marketplace as
`superpowers@claude-plugins-official`
([README](https://github.com/obra/superpowers/blob/22f0b463e7a5e579904dd6211a520321035526f7/README.md)).
We read its skills at commit `22f0b46`, from November 3, 2026.

- Brainstorming asks one question at a time, lays out approaches for the
  person to pick from, and writes a design doc the person approves
  before any plan is written
  ([brainstorming](https://github.com/obra/superpowers/blob/22f0b463e7a5e579904dd6211a520321035526f7/skills/brainstorming/SKILL.md)).
  It says to "never ask what the project or their context already
  answers, or reopen what they've already decided," so whatever our docs
  leave open is what it will ask about.
- Its test-driven development skill states "no production code without a
  failing test first," requires watching each test fail for the expected
  reason, and says code written before its test gets deleted
  ([test-driven-development](https://github.com/obra/superpowers/blob/22f0b463e7a5e579904dd6211a520321035526f7/skills/test-driven-development/SKILL.md)).
- Its guide to tests asks each test to name the break it catches and to
  exercise real code rather than mocks. It ends with a "mutation check":
  before finishing, the agent should "mentally mutate the production
  code," and at least one test should fail for each realistic mutation
  ([writing-good-tests](https://github.com/obra/superpowers/blob/22f0b463e7a5e579904dd6211a520321035526f7/skills/test-driven-development/writing-good-tests.md)).
  The check is done in the agent's head, not with a tool.
- Subagent-driven development gives each task in the plan to a fresh
  subagent, has a reviewer check each task for spec compliance and code
  quality, and ends with a review of the whole branch
  ([subagent-driven-development](https://github.com/obra/superpowers/blob/22f0b463e7a5e579904dd6211a520321035526f7/skills/subagent-driven-development/SKILL.md)).
  It's built to run unattended: it says not to pause between tasks, to
  decide ambiguities and record each decision in a ledger as a "Ruling,"
  and to stop only for destructive or security-sensitive actions, side
  effects outside the worktree, or a plan too broken to follow. The
  ledger lets a session recover after its context is compacted.

Superpowers writes its own design docs to `docs/superpowers/specs/` and
its plans to `docs/superpowers/plans/`, and its subagents commit as they
go. Both conflicted with rules this repo's `CLAUDE.md` had at the time,
which required approval of each commit message and kept work to do in
`PLAN.md`, so we removed those rules.

## Enforcing test-driven development with hooks

TDD Guard is a Claude Code plugin that uses hooks to block
"implementation without failing tests" and "code beyond current test
requirements." Its supported test frameworks include Go. Its README now
says it "grew into Probity," that "new projects should start there," and
that "TDD Guard remains maintained for the projects that rely on it"
([nizos/tdd-guard](https://hn.jakechampion.name/reader/https://github.com/nizos/tdd-guard)).

Probity "catches over-implementation and missing tests by reading your
agent's session history," works with Claude Code, Codex, and two Copilot
tools, and lets a person block commands and content by pattern. Its
listing showed 32 stars and a last push on May 2, 2026
([Probity](https://awesome.ecosyste.ms/projects/github.com%2Fnizos%2Fprobity)).

## Anthropic's own plugins

Anthropic's marketplace includes `code-review`, which "runs parallel
agents over a change set to catch bugs, missed edge cases, and style
issues before you commit"; `security-guidance`, which "reviews Claude's
diffs for injection, XSS, SSRF, exposed secrets, IDOR, and auth-bypass
patterns"; and language-server plugins, including one for Go that needs
the `gopls` binary
([DesignRevision, July 2026](https://designrevision.com/blog/official-claude-code-plugins)).

## Compound Engineering

Every's Compound Engineering plugin rests on the idea that "each unit of
engineering work should make subsequent units easier." Its loop is "Plan
→ Work → Review → Compound → Repeat," where the compound step records
what was learned, and its review command "spawns 14 specialized agents
that analyze code in parallel"
([Every](https://every.to/guides/compound-engineering)). Its guide
treats tests as a check after each change, not as something written
first.

## Anthropic's guidance on long-running agents

Anthropic's engineers described how they kept agents productive across
many sessions
([Anthropic, November 2025](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)):

- A first session writes a list of features, all marked failing. They
  "landed on using JSON for this, as the model is less likely to
  inappropriately change or overwrite JSON files," and told later agents
  to "edit this file only by changing the status of a passes field."
- A progress file "keeps a log of what agents have done."
- Each agent is "asked to work on only one feature at a time."
- Without prompting, Claude tested "with unit tests or `curl` commands"
  and missed features that didn't work end to end; told to "do all
  testing as a human user would," it did better.

Superpowers' ledger and per-task plan cover the first three. The fourth
matters for the catalog: an end-to-end test would run the daemon over a
sample tree and compare the `.catalog.md` it writes with one we've
checked.

## Mutation testing

Mutation testing changes the code in small ways and checks that some
test fails for each change. Thoughtworks moved it to "Adopt" in its
Technology Radar on November 4, 2026, because "as teams adopt AI coding
assistants, the volume of AI-generated code and tests increases
rapidly," and mutation testing helps "ensure generated tests don't
simply pass without catching logic changes"
([Thoughtworks](https://www.thoughtworks.com/radar/techniques/mutation-testing)).
For Go, Gremlins describes itself as "a mutation testing tool for Go";
it's in 0.x, and "only the current minor release will be maintained"
([Gremlins](https://awesome.ecosyste.ms/projects/github.com%2Fgo-gremlins%2Fgremlins)).

## What we chose

We're building with Claude Code and Superpowers. Superpowers already
covers most of how we want the work done: tests first, tests that catch
real faults, fresh reviewers, and a record that survives restarts. We
aren't committed to its method for writing tests. What matters is that
the tests catch faults, and a mutation testing tool such as Gremlins is
how we'd check that independently of the agent's own mental check.

## Unverified

- Miranda's benchmark figures come from benchmark sites he cites. We
  didn't check them against those sites.
- We read TDD Guard's README through a reader proxy and Probity and
  Gremlins through a package index, not their repositories directly.
- We haven't installed or run any of these tools.
