package profiler

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

Good (a real near-duplicate to disambiguate): "Bread-baking notes: oven temperatures, proofing times, and flour substitutions tried so far. Open it to see what was tried and how it turned out. A near-identical file lives at kitchen/bread_recipes.md — for the current recipes, use that one; this is the source it was built from."

Bad: "When the task involves understanding the full structure, pricing, and scope of West Arete's service tiers — for instance before drafting a proposal — this document should be loaded."

Speak plainly; this file is published and follows the same voice as the rest of the repo. Don't call a document "canonical," "authoritative," or "definitive" unless it says so itself. Describe only what the document's own text supports. Return only the profile — no heading, no path, no preamble.`
