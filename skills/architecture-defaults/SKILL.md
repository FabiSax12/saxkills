---
name: architecture-defaults
description: The owner's default project structure, layering and dependency policy for TypeScript/Node, React/Next.js and Python. Use when starting a project, adding a module or feature, deciding where code lives, or choosing whether to add a dependency.
---

# Architecture defaults

Existing project conventions override everything here. These are the defaults for new code and new projects.

## Core shape

Organize by **feature/domain**. Inside each feature, use **layers from day one**.

```
src/
  <feature>/
    <feature>.routes.ts      transport: HTTP, CLI, events (thin)
    <feature>.service.ts     business logic, no I/O details
    <feature>.repository.ts  data access, the only place that talks to storage
    <feature>.types.ts       types and schemas for this feature
    <feature>.test.ts
  shared/                    code used by 2+ features, nothing feature-specific
  config/                    env parsing and validation, one place
  main.ts                    composition root: wires dependencies, starts the app
```

- Dependencies point inward: transport, then service, then repository. Never the reverse.
- A feature talks to another feature only through its service's public functions, not its repository.
- `shared/` stays small. When something is only used by one feature, it lives in that feature.
- Wire dependencies in one place (the entry point). No hidden globals or service locators. See `solid`.
- Config is read and validated once at startup. The rest of the code receives typed values.

Layers are kept thin. If a layer only forwards calls, collapse it for that feature and note why.

## TypeScript / Node

- Strict mode on. No `any` without a reason in the same line.
- Validate external input at the boundary with a schema library, then trust the types inside.
- Named exports, one main concept per file.
- Async with `async/await`. Errors as typed error classes or result values, thrown at the edges. See `clean-code`.

## React / Next.js

- Feature folders mirror the backend: `features/<feature>/{components,hooks,api,types}`.
- Components stay presentational where possible. Data fetching and state live in hooks or server components.
- Server components by default in Next.js. Add `"use client"` only when interactivity requires it.
- Shared UI primitives in `components/ui`. No feature logic there.
- Colocate tests and styles with the component.

## Python

- `src/<package>/<feature>/` with the same layer split. Type hints on public functions.
- Dependency and environment management with a lockfile. Pin versions.
- Pydantic or dataclasses at the boundary for validation. Plain functions over classes where there is no state.
- Scripts are thin entry points that call library code, so everything is testable.

## Dependencies

- Minimal. Standard library first.
- Add a package only for clear value that would cost real effort to rebuild (crypto, parsing, protocol clients, UI frameworks).
- **Ask before adding a new dependency.** State the package, why, and the alternative.
- Prefer maintained, widely used packages with small dependency trees. Check size and last release.
- Pin versions and commit the lockfile.

## New project checklist

1. Feature folder layout above.
2. Formatter, linter and type checker configured and wired to one command.
3. Test runner and a first passing test.
4. Config module with validated env.
5. README with run, test and build commands.

## TODO(owner)

- Preferred web framework, ORM and test runner per stack.
- Database and migration tool defaults.
- Monorepo vs separate repos policy.
