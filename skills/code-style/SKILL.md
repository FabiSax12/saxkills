---
name: code-style
description: Personal code formatting and styling preferences. Use when writing, formatting or reviewing code to match the owner's layout, naming casing, import ordering, export style and file organization, plus TypeScript, Python and Java idioms. Defers to a project's own formatter and lint config when one exists.
---

# Code style

Items marked `TODO(owner)` are not decided yet. Do not guess them. Ask, or follow the project's existing style.

## Precedence

1. Project formatter and linter config (`.prettierrc`, `.editorconfig`, `ruff`, `gofmt`, etc.) always wins.
2. Existing style in the file being edited.
3. The preferences below.

Never reformat code you are not otherwise changing.

## Layout

- Indentation: one tab.
- Quotes: double.
- Semicolons: yes (JS/TS).
- Trailing commas: no.
- Line length: no limit. Do not wrap just to hit a width.
- Braces on the same line. Braces are required, except a single short statement may skip them: `if (!user) return null`. Anything longer than one statement gets braces.
- Early returns over nested conditionals. Max 2 levels of nesting.
- Blank line between logical steps inside a function, none inside tight groups.
- Names do the documenting. Almost no comments. See `self-documenting-code`.

## Naming

| Kind | Rule |
| --- | --- |
| Variables, functions | Follow the language: camelCase in JS/TS, snake_case in Python |
| Constants | `SCREAMING_SNAKE` for every primitive `const`, local or global |
| Files | PascalCase for components and classes, kebab-case for everything else |
| Folders | PascalCase for component folders, kebab-case for everything else |

```ts
const MAX_RETRIES = 3
const REQUEST_TIMEOUT_MS = 5000
```

```
components/
  LoginForm/
    LoginForm.tsx
    login-form.utils.ts
shared-utils/
  format-date.ts
```

Non-primitive constants (objects, arrays, functions) keep the normal casing of their kind.

## Imports and exports

- Group order: builtin, external, internal (aliases), relative. Alphabetical inside a group. No blank line between groups.
- Named exports by default. Use a default export only when the framework requires it (framework pages, config files).

```ts
import { readFile } from "node:fs/promises"
import { z } from "zod"
import { getUser } from "@/lib/users"
import { formatName } from "./format-name"
```

## File organization

- Exports per file: no rule. Do what fits the case.
- Types live in a `types.ts` per folder or feature.
- Tests live in a root `tests/` folder.

## TypeScript

- `interface` for object shapes whenever possible. `type` only when `interface` cannot express it (unions, mapped types).
- Top-level functions are `function` declarations. Arrows are for callbacks.
- `async/await` with `try/catch`. No `.then()` chains.

```ts
export function parseAge(input: string): number {
	const age = Number(input)
	if (Number.isNaN(age)) throw new Error("invalid age")
	return age
}

const ids = users.map((user) => user.id)
```

## Python

- Type hints on every function signature, public and private.
- Structured data uses Pydantic `BaseModel`.
- Formatter, linter and string style: no preference. Follow the project config.

```python
class User(BaseModel):
    name: str
    age: int


def find_adult(users: list[User]) -> User | None:
    return next((user for user in users if user.age >= 18), None)
```

## Java

- Immutable data uses `record` whenever possible. Use a class only when behavior or inheritance is needed.
- Null handling and exceptions: no preference. Follow the project.

```java
public record Point(int x, int y) {}
```

## TODO(owner)

- Casing for types, classes, interfaces and enums.
- Go idioms (error handling, package layout).
- Idioms for any other stack not listed above.
