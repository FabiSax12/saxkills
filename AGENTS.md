# AGENTS.md

This repo holds one person's private coding skills. Treat every skill as a statement of that person's opinions, not as generic best practice.

## Rules for editing this repo

- Do not invent preferences. If a skill needs a fact about how the owner works and it is not written down, leave a clearly marked `TODO(owner)` and say so in your reply.
- One skill per folder: `skills/<name>/SKILL.md`. The folder name equals the `name` in frontmatter (lowercase, hyphens).
- Frontmatter has exactly `name` and `description`. Nothing tool-specific. The description states what the skill does and when to use it, because that is what an agent reads to decide whether to load it.
- Body: imperative, short, examples over explanation. Under roughly 150 lines. Split into `references/*.md` inside the skill folder if it grows, and link from `SKILL.md`.
- Model agnostic: no model names, no tool names, no assumptions about slash commands or hooks.
- Artifacts are written in English.
- Keep the README skill table in sync with `skills/`.

## Rules the skills themselves follow

- Rules carry a reason or a clear trigger. A bare "always do X" with no context gets cut.
- Tiers are explicit: must, prefer, ignore. Do not blur them.
- No code comments in examples unless the example is about comments.
