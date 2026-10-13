# saxkills

Personal, private skills for AI coding agents. Model and tool agnostic.

Each skill is a folder with a single `SKILL.md` (YAML frontmatter + Markdown). That format is read by Claude Code, Codex, OpenCode, Gemini CLI and anything else that follows the Agent Skills convention. No tool-specific syntax lives inside the skills.

## Skills

| Skill | Purpose | Status |
| --- | --- | --- |
| [`clean-code`](skills/clean-code/SKILL.md) | My Clean Code for the AI era: musts, light-touch rules, and what to ignore. Includes daily habits like early returns. | draft |
| [`solid`](skills/solid/SKILL.md) | SOLID as a decision guide: when each principle pays off and when it is over-engineering. | draft |
| [`self-documenting-code`](skills/self-documenting-code/SKILL.md) | Names and structure do the explaining. Comments only when the code cannot say it. | draft |
| [`code-style`](skills/code-style/SKILL.md) | Formatting and styling preferences. | skeleton, needs your input |
| [`communication-style`](skills/communication-style/SKILL.md) | How the agent talks to you: language, length, tone, ambiguity. | interviewed |
| [`architecture-defaults`](skills/architecture-defaults/SKILL.md) | Project structure, layering and dependency policy for TS/Node, React/Next.js, Python. | interviewed |
| [`commit-and-pr-style`](skills/commit-and-pr-style/SKILL.md) | Conventional Commits, small frequent commits, one PR per feature. | interviewed |

"draft" means the content is a first pass written from the brief. Edit it until it says what you actually believe.

## Layout

```
skills/<skill-name>/SKILL.md   one skill per folder
scripts/install.sh             symlinks skills into an agent's skills directory
AGENTS.md                      rules for any agent editing this repo
```

## Install

Symlinks, so edits here apply immediately everywhere.

```bash
./scripts/install.sh claude        # ~/.claude/skills
./scripts/install.sh agents        # ~/.agents/skills (cross-tool convention)
./scripts/install.sh /any/path     # any skills directory
./scripts/install.sh claude clean-code solid   # only some skills
```

Existing entries that are not symlinks into this repo are never overwritten.

## Adding a skill

1. `mkdir skills/<name>` and create `SKILL.md`.
2. Frontmatter needs `name` (matches the folder) and `description` (says what it does and when to trigger).
3. Keep the body short and imperative. Add a row to the table above.
4. Re-run the install script.

See `AGENTS.md` for the authoring rules.
