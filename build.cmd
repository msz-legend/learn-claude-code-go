@echo off
setlocal
set GOFLAGS=-p=1
if not exist bin mkdir bin
for %%C in (s01_agent_loop s02_tool_use s03_permission s04_hooks s05_todo_write s06_subagent s07_skill_loading s08_context_compact s09_memory s10_task_system s11_background_tasks s12_cron_scheduler s13_agent_teams s14_mcp_plugin s15_integrated_harness s16_workflow_runtime s17_goal_loop) do (
  echo building %%C
  go build -o bin\%%C.exe .\%%C
  if errorlevel 1 exit /b 1
)
go build -o bin\mcp-demo.exe .\s14_mcp_plugin\demo
if errorlevel 1 exit /b 1
echo built 17 chapters + demo MCP server into bin\
