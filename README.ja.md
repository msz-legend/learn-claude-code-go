# learn-claude-code（Go）

[English](README.md) · [中文](README.zh.md) · [日本語](README.ja.md)

[learn-claude-code](https://github.com/shareAI-lab/learn-claude-code) の Go 移植版です。全 17 章で、素朴なエージェントループから始めて、権限、フック、メモリ、タスク、チーム、ワークフロー、ゴール判定までを順に積み上げます。

移植版は**標準ライブラリのみ**を使います。転送は `net/http`、プロトコルは `encoding/json`、ツールと MCP サーバーは `os/exec` です。SDK もモジュール依存もありません。

> このリポジトリは元の Python 講座の姉妹プロジェクトであり、置き換えではありません。まずここで章の概念を読み、最短の実装を見たいときに Python のソースと照らし合わせてください。

---

## 必要なもの

- Go 1.22 以降(`go version`)
- PowerShell 7、Windows PowerShell 5.1、または POSIX シェル
- モデルの提供元: Anthropic の API キー、または OpenAI 互換のエンドポイント

## クイックスタート

```powershell
Copy-Item .env.example .env
# .env を編集: LLM_PROTOCOL とどちらかの API キーを設定

go test ./...
go run ./s01_agent_loop
```

全章をまとめてビルドする場合:

```powershell
.\build.ps1
.\bin\s01_agent_loop.exe
```

macOS や Linux では PowerShell なしで同じ手順です:

```bash
cp .env.example .env
go build ./...
go run ./s01_agent_loop
```

どの章も `--help` に対応しており、モデルに問い合わせずにツールと調整項目を表示します。

## モデルの提供元

`LLM_PROTOCOL` が通信形式を選びます。未設定の場合、harness は存在するキーからプロトコルを推測します。

| プロトコル | 必須 | 一般的な base URL | モデル変数 |
|---|---|---|---|
| `anthropic` | `ANTHROPIC_API_KEY` | `https://api.anthropic.com` | `ANTHROPIC_MODEL` |
| `openai` | `OPENAI_API_KEY` | `https://api.openai.com/v1` | `OPENAI_MODEL` |

`MODEL_ID` は上記 2 つのモデル変数より優先されます。OpenAI 互換の例:

| 提供元 | `OPENAI_BASE_URL` | `MODEL_ID` |
|---|---|---|
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4.6` |
| Qwen | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` |
| Ollama | `http://localhost:11434/v1` | `llama3.2` |

## ディレクトリ構成

```
internal/harness/        共有実装
  loop.go                唯一のループ
  anthropic.go           Messages API クライアント
  openai.go              OpenAI 互換クライアント
  json.go                encoding/json の小さな補助
  tools.go               bash、read_file、write_file、edit_file、glob
  permission.go          deny / ask / allow ポリシー
  hooks.go               4 つのフックポイント
  compactor.go           アーカイブ、切り詰め、要約
  tasks.go               ファイルベースのタスクグラフ
  team.go                メールボックスとチームメイト
  workflow.go            ジャーナル付きステップ実行器
  goal.go                停止境界の独立評価器

s01_agent_loop/          各章に main パッケージ
s02_tool_use/
...
s17_goal_loop/
skills/                  s07 と s15 用の SKILL.md 例
```

実行時に作られる状態は `.memory/`、`.tasks/`、`.cron/`、`.compact/`、`.team/`、`.workflows/` に置かれ、git の対象外です。

## 章一覧

| 章 | 追加されるもの |
|---|---|
| [s01 Agent Loop](s01_agent_loop/README.md) | モデル → ツール → 結果 → 繰り返し。ツールは bash のみ。 |
| [s02 Tool Use](s02_tool_use/README.md) | 型付きディスパッチ表とワークスペース内に限定したファイルツール。 |
| [s03 Permission](s03_permission/README.md) | ツール実行前の deny/ask/allow 判定。 |
| [s04 Hooks](s04_hooks/README.md) | prompt、pre-tool、post-tool、stop の 4 フック。 |
| [s05 TodoWrite](s05_todo_write/README.md) | harness が管理する計画と、古くなった計画の通知。 |
| [s06 Subagent](s06_subagent/README.md) | 新しいコンテキストの子ループ。返すのはテキストだけ。 |
| [s07 Skill Loading](s07_skill_loading/README.md) | カタログをプロンプトに置き、本文は必要時に読み込む。 |
| [s08 Context Compact](s08_context_compact/README.md) | 層状の圧縮と引き継ぎ用の要約。 |
| [s09 Memory](s09_memory/README.md) | 永続的な事実の選択、抽出、統合。 |
| [s10 Task System](s10_task_system/README.md) | ディスク上の依存グラフとアトミックなクレーム。 |
| [s11 Background Tasks](s11_background_tasks/README.md) | 遅いコマンドを別スレッドへ移し、通知として受け取る。 |
| [s12 Cron Scheduler](s12_cron_scheduler/README.md) | 永続化した将来のプロンプトと 5 フィールドのパーサー。 |
| [s13 Agent Teams](s13_agent_teams/README.md) | チームメイト、共有ボード、追記専用メールボックス。 |
| [s14 MCP Plugin](s14_mcp_plugin/README.md) | stdio JSON-RPC によるツール発見と名前空間。 |
| [s15 Integrated Harness](s15_integrated_harness/README.md) | すべての仕組みを 1 つのループの周りに組み合わせる。 |
| [s16 Workflow Runtime](s16_workflow_runtime/README.md) | ホストが制御するステップと再開可能なジャーナル。 |
| [s17 Goal Loop](s17_goal_loop/README.md) | 停止境界に置く独立した評価器。 |

## 設計メモ

- **ループは 1 つ。** どの章も同じ `AgentLoop.Run` に到達します。章のエントリポイントはツールを選び、ポートを組み立てるだけです。
- **提供元に依存しないメッセージ。** `Msg` と `Block` がテキスト、ツール呼び出し、ツール結果を表し、各クライアントが自分の JSON 形式へ変換します。
- **失敗もメッセージ。** 拒否されたツールや未知のツールはエラーフラグ付きの `tool_result` を返すので、モデルが自分で立て直せます。ループは落ちません。
- **永続状態は普通のファイル。** データベースも隠れたデーモンもありません。harness が覚えているものはすべて確認、削除、差分比較できます。
- **教材コードは小さく保つ。** エラー処理は、黙ってデータを壊す箇所だけに入れます。製品向けの堅牢化はしません。

## 移植メモ

元の Python 講座は章ごとに独立したスクリプトです。この移植版は章のエントリポイントを薄くし、harness を共有することで、同じ仕組みを 17 回コピーしないようにしています。章により小さいツール群が必要なときは、別のレジストリを組みます。

インターフェースと小さな構造体を使い、Python 版と見比べやすい形にしています。並行処理は、それを主題とする章だけに登場します。バックグラウンドタスク、チーム、スケジューラです。

メモリの少ない Windows 環境では、並列リンクが `failed to create new OS thread` で失敗することがあります。`go build`、`go test`、`go vet` の前に `GOFLAGS=-p=1` を設定してください。

## ライセンス

MIT。元の講座と設計は shareAI Lab によるものです。[LICENSE](LICENSE) を参照してください。
