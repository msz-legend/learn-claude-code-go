# learn-claude-code（Go）

[English](README.md) · [中文](README.zh.md) · [日本語](README.ja.md)

这是 [learn-claude-code](https://github.com/shareAI-lab/learn-claude-code) 的 Go 版,共 17 章。课程从最朴素的 agent 循环开始,一步步加上权限、钩子、记忆、任务、团队、工作流和目标准入判断。

本移植**只用标准库**:`net/http` 负责传输,`encoding/json` 处理协议,`os/exec` 跑工具和 MCP 服务。没有 SDK,`go.mod` 里也没有任何依赖。

> 本仓库是原 Python 课程的姊妹项目,不是替代品。建议先在这里读某一章的概念,再对照 Python 源码看最短实现。

---

## 环境要求

- Go 1.22 或更高版本(`go version`)
- PowerShell 7、Windows PowerShell 5.1,或任意 POSIX shell
- 一个模型提供方:Anthropic API key,或任意 OpenAI 兼容端点

## 快速开始

```powershell
Copy-Item .env.example .env
# 编辑 .env:设置 LLM_PROTOCOL 和其中一个 API key

go test ./...
go run ./s01_agent_loop
```

一次编译全部章节:

```powershell
.\build.ps1
.\bin\s01_agent_loop.exe
```

macOS 或 Linux 上不需要 PowerShell:

```bash
cp .env.example .env
go build ./...
go run ./s01_agent_loop
```

每一章都支持 `--help`,会列出可用工具和调参项,不会请求模型。

## 模型提供方

`LLM_PROTOCOL` 决定协议。不设置时,harness 会根据实际存在的 key 推断。

| 协议 | 必需变量 | 常见 base URL | 模型变量 |
|---|---|---|---|
| `anthropic` | `ANTHROPIC_API_KEY` | `https://api.anthropic.com` | `ANTHROPIC_MODEL` |
| `openai` | `OPENAI_API_KEY` | `https://api.openai.com/v1` | `OPENAI_MODEL` |

`MODEL_ID` 的优先级高于上面两个模型变量。OpenAI 兼容端点的例子:

| 提供方 | `OPENAI_BASE_URL` | `MODEL_ID` |
|---|---|---|
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4.6` |
| Qwen | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` |
| Ollama | `http://localhost:11434/v1` | `llama3.2` |

## 目录结构

```
internal/harness/        共享实现
  loop.go                唯一的那个循环
  anthropic.go           Messages API 客户端
  openai.go              OpenAI 兼容客户端
  json.go                对 encoding/json 的少量封装
  tools.go               bash、read_file、write_file、edit_file、glob
  permission.go          deny / ask / allow 策略
  hooks.go               四个钩子点
  compactor.go           归档、截断、摘要
  tasks.go               基于文件的任务图
  team.go                邮箱和队友
  workflow.go            带日志的步骤执行器
  goal.go                独立的停止边界判断器

s01_agent_loop/          每章一个 main 包
s02_tool_use/
...
s17_goal_loop/
skills/                  s07 和 s15 用的 SKILL.md 示例
```

运行时产生的状态放在 `.memory/`、`.tasks/`、`.cron/`、`.compact/`、`.team/`、`.workflows/` 下,这些目录已被 git 忽略。

## 章节

| 章节 | 新增内容 |
|---|---|
| [s01 Agent Loop](s01_agent_loop/README.md) | 模型 → 工具 → 结果 → 再循环,只有一个 bash 工具。 |
| [s02 Tool Use](s02_tool_use/README.md) | 带类型的调度表和限定在工作区内的文件工具。 |
| [s03 Permission](s03_permission/README.md) | 工具执行前的 deny/ask/allow 检查。 |
| [s04 Hooks](s04_hooks/README.md) | prompt、pre-tool、post-tool、stop 四个钩子。 |
| [s05 TodoWrite](s05_todo_write/README.md) | harness 自己维护的计划,以及计划过期提醒。 |
| [s06 Subagent](s06_subagent/README.md) | 全新上下文的子循环,只把文本带回。 |
| [s07 Skill Loading](s07_skill_loading/README.md) | 目录进 prompt,正文按需加载。 |
| [s08 Context Compact](s08_context_compact/README.md) | 分层压缩,并留下一份交接摘要。 |
| [s09 Memory](s09_memory/README.md) | 持久事实的挑选、抽取和合并。 |
| [s10 Task System](s10_task_system/README.md) | 磁盘上的依赖图和原子领取。 |
| [s11 Background Tasks](s11_background_tasks/README.md) | 慢命令移到后台,以通知形式送回。 |
| [s12 Cron Scheduler](s12_cron_scheduler/README.md) | 持久化的定时提示,配五字段解析器。 |
| [s13 Agent Teams](s13_agent_teams/README.md) | 队友、共享看板、只追加的邮箱。 |
| [s14 MCP Plugin](s14_mcp_plugin/README.md) | stdio JSON-RPC 工具发现与命名空间。 |
| [s15 Integrated Harness](s15_integrated_harness/README.md) | 所有机制围绕同一个循环组合起来。 |
| [s16 Workflow Runtime](s16_workflow_runtime/README.md) | 宿主掌控的步骤,加可恢复的执行日志。 |
| [s17 Goal Loop](s17_goal_loop/README.md) | 停在停止边界上的独立判定器。 |

## 设计说明

- **只有一个循环。** 每章最终都落到同一个 `AgentLoop.Run`。章节入口只负责挑工具、组装端口。
- **与提供方无关的消息模型。** `Msg` 和 `Block` 表达文本、工具调用和工具结果,每个客户端各自翻译成对应的 JSON 结构。
- **失败也是消息。** 被拦截的工具和未知工具都会返回带错误标记的 `tool_result`,让模型能自己纠正,而不是把循环打挂。
- **持久状态就是普通文件。** 没有数据库,也没有隐藏的守护进程。harness 记住的东西都能直接看、删或 diff。
- **教学代码保持小。** 只在会悄悄损坏数据的地方做错误处理,不按生产标准加固。

## 移植说明

原 Python 课程每章是一个独立脚本。这里把章节入口写薄,共享一套 harness,避免同一个机制复制 17 份。某章需要更小的工具面时,单独建一个注册表。

接口和小结构体的写法尽量贴近 Python 原版,方便对照。并发只出现在真正讲并发的章节:后台任务、团队和调度器。

在内存紧张的 Windows 机器上,并行链接可能报 `failed to create new OS thread`。执行 `go build`、`go test`、`go vet` 前先设 `GOFLAGS=-p=1`。

## 许可

MIT。原课程与设计归 shareAI Lab。见 [LICENSE](LICENSE)。
