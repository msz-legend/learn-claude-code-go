$ErrorActionPreference = "Stop"

# Build every chapter into bin/. The Go linker on Windows is memory hungry;
# -p=1 keeps parallel linking from exhausting the paging file.
$env:GOFLAGS = "-p=1"

$chapters = @(
    "s01_agent_loop",
    "s02_tool_use",
    "s03_permission",
    "s04_hooks",
    "s05_todo_write",
    "s06_subagent",
    "s07_skill_loading",
    "s08_context_compact",
    "s09_memory",
    "s10_task_system",
    "s11_background_tasks",
    "s12_cron_scheduler",
    "s13_agent_teams",
    "s14_mcp_plugin",
    "s15_integrated_harness",
    "s16_workflow_runtime",
    "s17_goal_loop"
)

New-Item -ItemType Directory -Force bin | Out-Null
foreach ($chapter in $chapters) {
    $name = $chapter
    Write-Host "building $chapter"
    go build -o "bin\$name.exe" "./$chapter"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
go build -o "bin\mcp-demo.exe" "./s14_mcp_plugin/demo"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "built 17 chapters + demo MCP server into bin/"
