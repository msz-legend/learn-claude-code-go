---
name: code-review
description: Review a diff or a small module for correctness, tests, and clarity. Use before merging or when the user asks for a review.
---

# Code Review

Read the actual files first. A review based on names or guesses is worse than
no review.

## Passes

1. **Correctness** - trace each changed branch. Look for nil/zero values,
   off-by-one errors, swallowed errors, and state that escapes its guard.
2. **Tests** - does a test fail if the change is reverted? If not, the test
   does not cover the change.
3. **Clarity** - names match behavior, comments explain why, and the diff does
   not carry unrelated edits.

## Output

Group findings by severity:

- **blocker** - wrong result, data loss, crash, or security issue
- **should-fix** - a real bug in an edge case, or a missing test
- **nit** - style only

Every finding needs a file, a line, and a concrete suggested change. If there
are no findings, say so and name what you checked.
