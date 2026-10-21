# s15: 統合ハーネス

[English](README.md) · [中文](README.zh.md) · [日本語](README.ja.md)

[s14: MCP プラグイン](../s14_mcp_plugin/) → **s15** → [s16: ワークフローランタイム](../s16_workflow_runtime/)

> 1つのループ、すべてのポート。仕組みは組み合わさって初めて面白い。

---

## 課題

個々の仕組みは小さい。本当のハーネスの問いは、権限・計画・メモリ・タスク・通知・圧縮が同じターンをどう共有するかだ。

## 仕組み

1. 全ツールを1つのレジストリに接続し、子のツールは小さく保つ。
2. 権限、フック、圧縮、通知を1つのループの周りで組み合わせる。
3. /status を用意し、ハーネスが何を保持しているか見せる。

## コードマップ

| File | Role |
|---|---|
| `s15_integrated_harness/main.go` | 本章の入口。この仕組みを1つのループに接続する。 |
| `internal/harness/loop.go`, `internal/harness/todo.go`, `internal/harness/skills.go`, `internal/harness/memory.go`, `internal/harness/tasks.go`, `internal/harness/background.go`, `internal/harness/cron.go`, `internal/harness/context.go`, `internal/harness/agents.go` | 全章で共有する実装。 |
| `.tasks/, .memory/, .cron/, .compact/` | 本章が書き出す状態。 |

## 実行

```bash
go run ./s15_integrated_harness
```

初回実行前に .env.example を .env にコピーし、いずれかのプロバイダーを設定してください。認証情報がなくても --help は動きます。

## 試してみる

- 複数ステップの作業を頼み、/status と /tasks を見る。
- 遅いコマンドを走らせ、質問を続け、通知が届くのを待つ。

## 次章

次は [s16: ワークフローランタイム](../s16_workflow_runtime/README.ja.md) へ。
