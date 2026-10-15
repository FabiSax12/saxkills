# saxkills

Personal, private skills for AI coding agents. Model and tool agnostic.

Each skill is a folder with a single `SKILL.md` (YAML frontmatter + Markdown). That format is read by Claude Code, Codex, OpenCode, Gemini CLI and anything else that follows the Agent Skills convention. No tool-specific syntax lives inside the skills.

## Skills

| Skill | Purpose | Status |
| --- | --- | --- |
| [`clean-code`](skills/clean-code/SKILL.md) | My Clean Code for the AI era: musts, light-touch rules, and what to ignore. Includes daily habits like early returns. | draft |
| [`solid`](skills/solid/SKILL.md) | SOLID as a decision guide: when each principle pays off and when it is over-engineering. | draft |
| [`self-documenting-code`](skills/self-documenting-code/SKILL.md) | Names and structure do the explaining. Comments only when the code cannot say it. | draft |
| [`code-style`](skills/code-style/SKILL.md) | Formatting and styling preferences. | mostly done, small TODO(owner) left |
| [`communication-style`](skills/communication-style/SKILL.md) | How the agent talks to you: language, length, tone, ambiguity. | interviewed |
| [`architecture-defaults`](skills/architecture-defaults/SKILL.md) | Project structure, layering and dependency policy for TS/Node, React/Next.js, Python. | mostly done, small TODO(owner) left |
| [`commit-and-pr-style`](skills/commit-and-pr-style/SKILL.md) | Conventional Commits, small frequent commits, one PR per feature. | interviewed |

"draft" means the content is a first pass written from the brief. Edit it until it says what you actually believe.

## Layout

```
skills/<skill-name>/SKILL.md   one skill per folder
scripts/install.sh             legacy symlink flow, for editing skills locally
cmd/saxkills/                  entrypoint of the saxkills installer
internal/                      installer packages (github, skills, manifest, install, update, selfupdate, cli, tui)
.goreleaser.yaml               release build config
.github/workflows/release.yml  tests, then builds and attaches binaries when a v* tag is pushed
.github/workflows/ci.yml       vet, gofmt and race tests on push and pull requests
AGENTS.md                      rules for any agent editing this repo
```

## Install with the saxkills TUI

`saxkills` is a terminal app that copies these skills into a project or into your home directory and keeps them updated against the GitHub releases of this repo. Installed copies are pinned to a release and recorded in a `.saxkills.json` manifest next to the skills, so it only ever updates or removes skills it installed itself.

### Get the binary

This repo is private, so both options need GitHub credentials. saxkills reads `GITHUB_TOKEN` first, then the output of `gh auth token`, then falls back to anonymous requests (which work only if the repo becomes public). The token is never printed or logged.

```bash
export GITHUB_TOKEN=...            # or run: gh auth login

GOPRIVATE=github.com/FabiSax12/saxkills \
  go install github.com/FabiSax12/saxkills/cmd/saxkills@latest
```

`go install` also needs git access to the private repo (for example an SSH key, or a git credential helper that uses your token). Alternatively download a release binary:

```bash
gh release download --repo FabiSax12/saxkills --pattern 'saxkills_linux_amd64.tar.gz' --pattern checksums.txt
sha256sum --ignore-missing -c checksums.txt
tar -xzf saxkills_linux_amd64.tar.gz saxkills
install -m 0755 saxkills ~/.local/bin/saxkills
```

Assets are named `saxkills_<os>_<arch>.tar.gz` for linux and darwin on amd64 and arm64. Once installed, `saxkills self-update` replaces the binary with the latest release after verifying `checksums.txt`.

### Use it

Run `saxkills` with no arguments. Screens: Home (Install, Update, Remove, Status, Quit), location picker, skill list, confirmation, progress, result.

- Locations: `global` or `project`, for `claude` or `agents`. Several can be selected at once, and the resolved paths are shown.

  | | claude | agents |
  | --- | --- | --- |
  | global | `~/.claude/skills` | `~/.agents/skills` |
  | project | `<project>/.claude/skills` | `<project>/.agents/skills` |

- The project root is `git rev-parse --show-toplevel` from the current directory, or the current directory when there is no repo. Your home directory is never treated as a project.
- Skill list keys: `space` toggle, `a` all, `n` none, `enter` confirm, `esc` back, `q` quit.
- Each skill shows a badge: not installed, installed vX, update available vY, modified locally, conflict, linked (legacy), removed upstream.
- A directory that exists but is not in the manifest is a conflict and is never overwritten or removed.
- A managed skill you edited by hand is marked modified locally. Update and remove skip it unless you confirm overwriting it.
- Symlinks created by `scripts/install.sh` show as linked (legacy). Selecting them replaces the symlink with a managed copy.
- On launch it checks for a newer release in the background and shows banners for a newer binary and for outdated installs. Offline, it shows local state from the manifests and disables installing new skills.

### Commands

```bash
saxkills                                    # TUI
saxkills update --all                       # update every managed install (global and project, claude and agents)
saxkills update --scope global --target claude
saxkills update --all --quiet               # print errors only
saxkills check                              # exit code 10 when any managed install is outdated
saxkills self-update                        # replace the binary with the latest release
saxkills version
```

`update` needs `--all`, `--scope` or `--target`. Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | nothing to do, or everything succeeded |
| 1 | a failure (network, unreadable install, refused location); other installs are still processed |
| 2 | usage error |
| 3 | some skills were skipped (modified locally, removed upstream); a warning is printed even with `--quiet` |
| 10 | `check` only: at least one skill can be updated |

`check` ignores skills that `update` cannot change and lists them as "needs attention" instead (exit 0 for those alone). Skills removed upstream are reported, never deleted automatically. Project locations whose `.claude` or `.agents` folder is a symlink resolving outside the project are refused.

### Keep skills updated

systemd user timer (`~/.config/systemd/user/saxkills-update.service` and `.timer`):

```ini
[Unit]
Description=Update saxkills skills

[Service]
Type=oneshot
Environment=GITHUB_TOKEN=...
ExecStart=%h/.local/bin/saxkills update --all --quiet
```

```ini
[Unit]
Description=Daily saxkills update

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

```bash
systemctl --user enable --now saxkills-update.timer
```

A systemd service does not see your `gh` login, so put the token in `Environment=` or an `EnvironmentFile=` that only you can read.

Shell startup (bash or zsh), in the background so the prompt is not delayed:

```bash
(saxkills update --all --quiet >/dev/null 2>&1 &)
```

### Cut a release

```bash
git tag vX.Y.Z && git push --tags
```

The `release` workflow first checks that the tagged commit is on `main` and runs the tests, then builds the linux and darwin binaries with GoReleaser, and attaches them plus `checksums.txt` to the release. The tag is the version recorded in manifests, and the skills installed are the contents of `skills/` at that tag.

### Security notes

- `checksums.txt` only detects a corrupted or truncated download. It does not prove who published the release, so a compromised release or token would still pass `self-update` verification.
- TODO(owner): decide whether to sign releases (cosign/minisign/attestations).
- Skills are copied from the release tarball as plain files; the extractor rejects path traversal, symlinks and oversized archives.

## Install with symlinks (legacy)

`scripts/install.sh` links skills straight from a local clone, so edits apply immediately everywhere. Use it while editing skills; use the TUI to consume them.

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
4. Re-run the install script, or tag a release and run `saxkills update --all`.

See `AGENTS.md` for the authoring rules.
