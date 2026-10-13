---
name: self-documenting-code
description: Rules for code that explains itself through naming, structure and low complexity, with comments used sparingly. Use when writing or reviewing code and deciding whether to add a comment, how to name something, or when cleaning out redundant, obvious or over-explained comments.
---

# Self-documenting code

The code is the documentation. A comment is a small admission that the code could not say it. Try to fix the code first.

## Order of attack

1. **Rename.** A better name removes most comments.
2. **Extract.** Pull a block into a function named for what it does. The name replaces the comment.
3. **Name the condition.** `if (user.age >= 18 && user.hasConsent)` becomes `if (canRegister(user))`.
4. **Use types.** Enums, value types and return types state constraints a comment would.
5. **Only then** consider a comment.

## When a comment is allowed

Write a comment only for what code cannot express:

- **Why**, not what: a non-obvious business rule, a workaround for a specific bug, a performance trade-off, a regulatory constraint.
- A link to the issue, spec or upstream bug that justifies odd code.
- Warnings about consequences ("order matters: cache must be cleared before migrate").
- Public API docs for a published library, kept to behavior and contract.
- Legal headers when required.

A good "why" comment survives a rewrite of the code beneath it.

## Delete on sight

- Restating the code: `// increment i`, `// returns the user`.
- Section banners and dividers.
- Commented-out code.
- Changelog or authorship in comments. That is version control's job.
- TODOs with no owner or ticket.
- Narration of the agent's own process ("now we validate", "added to fix X").
- Over-explained comments longer than the code they annotate.
- Docstrings that only repeat the signature.

## Naming

- Functions: verb phrases (`calculateInvoiceTotal`). Booleans: questions (`isExpired`, `hasAccess`).
- Names reflect domain language and match what the team and the business say.
- Longer and clear beats short and cryptic. Scope sets length: a loop index can be `i`, a module-level value cannot.
- No abbreviations unless universal (`id`, `url`).
- A name that needs "and" or "or" signals a function doing too much.
- If you cannot name it, you do not understand it yet. Stop and clarify.

## Structure

- Early returns keep the happy path readable.
- Keep functions at one level of abstraction so the body reads like a short outline.
- Cyclomatic complexity stays low. Over about 7 branches, split.
- Extract constants for meaningful values (`MAX_RETRIES = 3`).
- Tests named as sentences document behavior better than any prose.

## Example

Before:

```ts
// check if user can get discount
if (u.t === 2 && u.o > 5 && !u.b) {
  // apply 10%
  p = p * 0.9;
}
```

After:

```ts
const DISCOUNT_RATE = 0.1;

if (qualifiesForLoyaltyDiscount(user)) {
  price = applyDiscount(price, DISCOUNT_RATE);
}
```

Here the `why` (for example, a policy that loyalty means 5+ orders) belongs inside `qualifiesForLoyaltyDiscount` only if the rule is not obvious from its name.

## Final check

For each comment: could a rename, an extraction or a type say this? If yes, change the code and delete the comment. If the comment explains why, keep it short.
