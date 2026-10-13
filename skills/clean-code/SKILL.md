---
name: clean-code
description: Personal Clean Code rules for writing and reviewing code in any language. Use when writing, refactoring or reviewing code, naming things, splitting functions, or deciding how much structure a change needs. Covers what is mandatory, what is light-touch, and what to ignore, plus daily habits such as early returns.
---

# Clean Code (AI era)

Code is read by humans and by agents, and rewritten constantly. Optimize for the next reader arriving cold, with grep and a small context window.

## Must

1. **Names carry intent.** A name says what the thing is or does in domain terms. No `data`, `info`, `handle`, `process`, `util`, `manager` unless that is genuinely the concept.
2. **Early returns.** Handle invalid and edge cases first, return, and keep the happy path unindented. Max nesting of 2 levels.
3. **One level of abstraction per function.** A function either orchestrates steps or does detail work, not both.
4. **Low complexity.** If a function needs a mental stack to follow, split it or flatten it. Prefer flat sequences over nested conditionals.
5. **No magic values.** Named constants or enums for anything with meaning.
6. **Explicit over clever.** No tricks that save lines and cost a reread. Readable beats short.
7. **Errors are handled or propagated on purpose.** Never swallow. Fail fast with a message that names the cause.
8. **No dead code.** Delete it. Git remembers. No commented-out blocks, no unused params, no "just in case" branches.
9. **Match the surrounding code.** Local conventions win over personal taste inside someone else's file.
10. **Small, focused diffs.** A change does one thing. Refactors and behavior changes do not share a commit.
11. **Tests describe behavior.** Test names read as specs. Test the contract, not the implementation.

## Prefer

- Pure functions and immutable data where the language makes it cheap.
- Types as documentation. Make illegal states unrepresentable before writing validation for them.
- Duplicate until the third occurrence, then extract. Wrong abstractions cost more than repetition.
- Locality: keep related code close. Fewer files to open beats a perfect layer diagram.
- Boring, standard-library solutions before new dependencies.
- Functions with 3 or fewer parameters. Past that, pass a named object.
- Positive conditions and affirmative names (`isReady`, not `isNotReady`).

## Ignore

- Hard line limits ("functions under 5 lines"). Cohesion matters, not the count.
- One assert per test as dogma.
- Interfaces, factories or layers created "for flexibility" with one implementation.
- Getter/setter ceremony. Use plain data where there is no invariant.
- Doc comments on every function. See `self-documenting-code`.
- Hungarian notation and type prefixes.
- Formatting debates. A formatter owns that. See `code-style`.

## Agent-specific

- Do not add speculative abstractions, config flags or extension points nobody asked for.
- Do not narrate your own work in code ("// now we loop"). No AI-flavored comments.
- Keep names greppable and unique enough to find with one search.
- Prefer the minimal change that satisfies the request. Mention adjacent problems, do not fix them silently.
- Read the neighbors before writing: reuse existing helpers instead of adding a near-duplicate.

## Example: early returns

Before:

```ts
function shippingCost(order: Order) {
  if (order) {
    if (order.items.length > 0) {
      if (!order.isDigital) {
        return calculate(order);
      } else {
        return 0;
      }
    } else {
      return 0;
    }
  }
  throw new Error("order required");
}
```

After:

```ts
function shippingCost(order: Order) {
  if (!order) throw new Error("order required");
  if (order.items.length === 0) return 0;
  if (order.isDigital) return 0;

  return calculate(order);
}
```

## Review checklist

Run through before calling work done: names honest, nesting flat, no dead code, no magic values, errors handled, diff focused, nothing speculative added.
