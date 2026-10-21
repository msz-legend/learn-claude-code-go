package harness

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	Reset  = "\u001b[0m"
	Cyan   = "\u001b[36m"
	Yellow = "\u001b[33m"
	Red    = "\u001b[31m"
	Dim    = "\u001b[2m"
)

var stdin = bufio.NewReader(os.Stdin)

func Color(text, code string) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}
	return code + text + Reset
}

func Header(chapter, title string) {
	fmt.Println(Color(chapter+": "+title, Cyan))
	fmt.Println(Color("model: "+DescribeModel(), Dim))
	fmt.Println(Color("workspace: "+Workdir(), Dim))
	fmt.Println()
}

func REPL(prompt string, loop *AgentLoop, history *[]Msg, handler func(string) bool) {
	for {
		line, ok := ReadLine(Color(prompt+" >> ", Cyan))
		if !ok {
			break
		}
		query := strings.TrimSpace(line)
		if query == "" || strings.EqualFold(query, "q") || strings.EqualFold(query, "exit") {
			break
		}
		if strings.HasPrefix(query, "/") && handler != nil && handler(query) {
			continue
		}
		*history = append(*history, User(query))
		text, err := loop.Run(history)
		if err != nil {
			fmt.Println(Color("Error: "+err.Error(), Red))
			continue
		}
		if strings.TrimSpace(text) != "" {
			fmt.Println(text)
		}
		fmt.Println()
	}
	fmt.Println("bye.")
}

func ReadLine(prompt string) (string, bool) {
	fmt.Print(prompt)
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

func ConsoleApprover(call Block, reason string) bool {
	fmt.Println()
	fmt.Println(Color("permission requested: "+reason, Yellow))
	fmt.Println("  " + call.Name + " " + Shorten(Stringify(call.Input), 400))
	answer, ok := ReadLine("  allow? [y/N] ")
	if !ok {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y")
}

func PrintingListener() *Listener {
	return &Listener{
		OnToolStart: func(call Block) {
			fmt.Println(Color("> "+call.Name+" "+Shorten(Stringify(call.Input), 240), Yellow))
		},
		OnToolResult: func(call Block, output string, isError bool) {
			prefix := "  "
			if isError {
				prefix = "  ! "
			}
			lines := strings.Split(Shorten(output, 600), "\n")
			fmt.Println(Color(prefix+strings.Join(lines, "\n  "), Dim))
		},
		OnToolBlocked: func(call Block, reason string) {
			fmt.Println(Color("  x "+reason, Red))
		},
	}
}

func Shorten(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r", " "))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
