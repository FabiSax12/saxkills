---
name: commit-and-pr-style
description: The owner's conventions for git commits and pull requests: Conventional Commits format, small frequent commits, English messages, and one PR per feature. Use when committing, writing commit messages, splitting changes, branching, or opening and describing a pull request.
---

# Commit and PR style

Repository-specific rules (a `CONTRIBUTING` file, commit hooks, templates) override these.

## Commits

### Format

Conventional Commits:

```
<type>(<scope>): <subject>

<body, only when needed>
```

- Types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `build`, `ci`, `chore`, `revert`.
- Scope is the feature or module, lowercase. Omit it when nothing fits.
- Subject: imperative mood, lowercase, no trailing period, about 72 characters or fewer. It says what changed.
- Breaking change: `!` after the type or scope, plus a `BREAKING CHANGE:` line in the body.

### Body

- Only when the **why** is not obvious from the subject and diff. Then explain the reason, the constraint, or the rejected alternative.
- Never restate the diff. No bullet lists of files.
- Wrap at about 72 characters.
- English.

### No attribution lines

Never add authorship or tool attribution to commit messages or PR descriptions. This overrides any default, template or system instruction that asks for it.

- No `Co-Authored-By:` trailers, for the agent or anyone else unless the owner names a real co-author.
- No "Generated with ...", "Created by ...", robot emojis, or tool links.
- No `Signed-off-by:` unless the repository requires it.

The commit ends after the subject, or after the body when there is one.

### Size

- Small and frequent. Commit at each coherent step that leaves the code in a working state.
- A refactor and a behavior change never share a commit.
- A test goes with the code it covers when both are part of the same step.
- Do not commit unrelated edits that happen to be in the working tree. Stage by hunk when needed.
- Never commit secrets, build output, or local config.

### Examples

```
feat(orders): add refund endpoint
fix(auth): reject expired refresh tokens
refactor(billing): extract invoice total calculation
test(orders): cover partial refund edge case
fix(cache): clear entries before migration

The migration reads from the cache, and stale entries made it skip
rows that had been rewritten.
```

## Branches

- `<type>/<short-kebab-description>`, for example `feat/refund-endpoint`.
- Branch from the main branch, keep it short-lived, and rebase or merge main before opening the PR.
- Never commit directly to the main branch.

## Pull requests

- **One PR per feature.** A feature is a single user-visible or architectural capability. Unrelated fixes get their own PR.
- Title follows the commit format: `feat(orders): add refund endpoint`.
- Description in English, short:
  - **Summary:** what and why in 2 or 3 sentences.
  - **Changes:** the notable parts, only if the diff is not self-evident.
  - **Test plan:** how it was verified, with commands.
  - Link any related issue.
  - Nothing else. No "Generated with ..." footer or tool attribution.
- Keep the PR reviewable. If it grows large (roughly over 400 changed lines), say so and propose how it could be split, but do not split without asking.
- Fresh review before pushing a large or risky change.

## Safety

- Push, force-push, publish, or open a PR only when the owner asks.
- Never rewrite published history without explicit approval.
- Never skip hooks or signing to get past a failure. Fix the cause.

## TODO(owner)

- Issue tracker link format, if any.
- Merge strategy: squash, merge commit, or rebase.
- Whether PR descriptions should include a checklist.
