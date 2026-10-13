---
name: code-style
description: Personal code formatting and styling preferences. Use when writing, formatting or reviewing code to match the owner's layout, naming casing, import ordering and file organization. Defers to a project's own formatter and lint config when one exists.
---

# Code style

Status: skeleton. Items marked `TODO(owner)` are not decided yet. Do not guess them. Ask, or follow the project's existing style.

## Precedence

1. Project formatter and linter config (`.prettierrc`, `.editorconfig`, `ruff`, `gofmt`, etc.) always wins.
2. Existing style in the file being edited.
3. The preferences below.

Never reformat code you are not otherwise changing.

## Decided (from stated habits)

- Early returns over nested conditionals.
- Flat code: max 2 levels of nesting.
- Names do the documenting. Almost no comments. See `self-documenting-code`.
- Blank line between logical steps inside a function, none inside tight groups.

## TODO(owner)

Fill these in. One line each is enough.

- Indentation: tabs or spaces, width.
- Quotes: single or double.
- Semicolons (JS/TS).
- Line length limit.
- Trailing commas.
- Brace style and single-line `if` without braces.
- Naming case per kind: variables, functions, types, constants, files, folders.
- Import ordering and grouping (builtin, external, internal, relative); named vs default exports.
- File organization: one export per file, where types live, test file placement.
- Preferred language idioms per stack (TypeScript, Python, Go, etc.).
- Commit message format.

## How to finish this skill

Paste a file you consider perfectly styled, or answer the list above, and it gets turned into short rules with one example each.
