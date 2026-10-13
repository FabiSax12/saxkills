---
name: solid
description: Personal guide to applying SOLID principles with concrete use cases and anti-patterns. Use when designing classes, modules or services, deciding whether to introduce an interface or abstraction, splitting responsibilities, or reviewing code for over- or under-engineering.
---

# SOLID: decision guide

SOLID is a set of pressure valves, not a checklist. Apply a principle when its pain is present or clearly imminent. Without that pain, the simpler code wins.

Rule of thumb: **introduce the abstraction on the second real use case, not the first imagined one.**

## S: Single Responsibility

One reason to change. Judge by who asks for changes, not by method count.

- **Apply when:** a module mixes concerns that change for different reasons (pricing rules plus persistence plus email formatting).
- **Smell:** class name needs "And" or "Manager"; tests need many unrelated mocks.
- **Skip when:** splitting produces pass-through classes that only forward calls.

## O: Open/Closed

Add behavior by adding code, not by editing a growing `if/else` or `switch` on a type.

- **Apply when:** the same type switch appears in 2+ places, or new variants arrive regularly (payment providers, export formats, notification channels). Use a strategy map or polymorphism.
- **Skip when:** there are 2 stable cases. A plain `if` is clearer than a registry.

## L: Liskov Substitution

A subtype must work anywhere its parent works, without callers checking which one they got.

- **Apply when:** using inheritance or implementing an interface. Do not strengthen preconditions, weaken guarantees or throw "not supported".
- **Smell:** `instanceof` checks, empty or throwing overrides.
- **Fix:** prefer composition. Inherit only for a true "is-a" with shared behavior.

## I: Interface Segregation

Callers depend only on what they use.

- **Apply when:** an interface forces implementers to stub methods, or a consumer needs one method of a big contract. Define the small interface at the consumer.
- **Skip when:** a single cohesive contract is always used whole.

## D: Dependency Inversion

Business logic depends on abstractions it owns. I/O details (database, HTTP, clock, filesystem, third-party SDKs) plug in at the edges.

- **Apply when:** domain logic touches I/O, or you need a test seam, or a vendor might change. Inject through the constructor or function argument.
- **Skip when:** the dependency is stable and cheap to use directly (standard library, pure helpers). Do not wrap them.
- **Wiring:** compose dependencies in one place (the entry point). No service locators, no hidden globals.

## Use cases

| Situation | Reach for |
| --- | --- |
| Many payment providers or export formats | O + D: one interface, one implementation per provider, chosen at the edge |
| Fat service class with unrelated methods | S: split by reason to change |
| Need to unit test logic that calls an API | D: inject a small port, fake it in tests |
| Subclass overrides a method to throw | L: drop inheritance, compose |
| Interface with 12 methods, consumers use 2 | I: consumer-owned narrow interface |
| One implementation, no variation expected | None of these. Write the concrete code. |

## Over-engineering alarms

- Interface with a single implementation and no test need.
- Abstract base class that only one class extends.
- "Factory" that calls one constructor.
- Layers that rename fields and pass them along.

If an alarm fires, inline it back to concrete code.

## In functional or non-OOP code

The same ideas hold: small functions with one reason to change (S), behavior passed as functions instead of switches (O/D), narrow function signatures (I). Replace classes and interfaces with function types and modules.
