param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Chapter
)

$env:GOFLAGS = "-p=1"
go run "./$Chapter"
