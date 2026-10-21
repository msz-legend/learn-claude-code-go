# s05: TodoWrite

[English](README.md) · [中文](README.zh.md) · [日本語](README.ja.md)

[s04: Hook System](../s04_hooks/) → **s05** → [s06: Subagent](../s06_subagent/)

> Keep the plan in the harness, where it can be shown again.

---

## The Problem

Long tasks drift: the model forgets what it planned, repeats work, or silently drops a step. A plan that only lives in the prompt disappears with the next summary.

## How It Works

1. Store an ordered list with pending, in_progress, and completed states.
2. Allow at most one item in progress so the next action is unambiguous.
3. Count tool rounds and re-inject the plan when it goes stale.

## Code Map

| File | Role |
|---|---|
| `s05_todo_write/main.go` | Chapter entry point: wires this mechanism into one loop. |
| `internal/harness/todo.go`, `internal/harness/hooks.go` | Shared implementation used by every chapter. |

## Run

```bash
go run ./s05_todo_write
```

Copy .env.example to .env and fill in one provider before the first run. --help works without credentials.

## Try This

- Ask for a three-step task and run /todo after each step.
- Skip todo updates for several rounds and watch the reminder appear.

## Next

Continue to [s06: Subagent](../s06_subagent/README.md).
