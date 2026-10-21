package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// ContextCompactor implements four layers of context control, cheapest first.
type ContextCompactor struct {
	archiveDir       string
	toolResultBudget int
	summarizeAt      int
	keepMessages     int
	compactions      int
}

func NewContextCompactor(workspace string) *ContextCompactor {
	return &ContextCompactor{
		archiveDir:       filepath.Join(workspace, ".compact"),
		toolResultBudget: GetInt("COMPACT_TOOL_RESULT_BUDGET", 200_000),
		summarizeAt:      GetInt("COMPACT_SUMMARIZE_AT_TOKENS", 24_000),
		keepMessages:     GetInt("COMPACT_KEEP_MESSAGES", 8),
	}
}

func (c *ContextCompactor) Compactions() int { return c.compactions }

func (c *ContextCompactor) BeforeRequest(history *[]Msg, client Client) error {
	if len(*history) == 0 {
		return nil
	}
	c.budgetToolResults(history)
	c.truncateToolResults(history, 4_000, 2_000)
	if EstimateTokens(*history) > c.summarizeAt {
		if err := c.summarize(history, client); err != nil {
			return err
		}
	}
	if EstimateTokens(*history) > c.summarizeAt*2 {
		c.truncateToolResults(history, 1_200, 600)
	}
	return nil
}

func EstimateTokens(history []Msg) int {
	chars := 0
	for _, message := range history {
		chars += len(message.Role)
		for _, block := range message.Blocks {
			switch block.Type {
			case BlockText:
				chars += len(block.Text)
			case BlockToolUse:
				chars += len(block.Name) + len(Stringify(block.Input))
			case BlockToolResult:
				chars += len(block.Content)
			}
		}
	}
	tokens := chars / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func Transcript(history []Msg, maxChars int) string {
	var out strings.Builder
	for _, message := range history {
		if out.Len() >= maxChars {
			break
		}
		out.WriteString("[" + message.Role + "]\n")
		for _, block := range message.Blocks {
			switch block.Type {
			case BlockText:
				out.WriteString(block.Text + "\n")
			case BlockToolUse:
				out.WriteString("tool_use " + block.Name + " " + Stringify(block.Input) + "\n")
			case BlockToolResult:
				out.WriteString("tool_result " + block.ToolUseID + ": " + block.Content + "\n")
			}
		}
		out.WriteString("\n")
	}
	text := out.String()
	if len(text) > maxChars {
		text = Truncate(text, maxChars) + "\n... (transcript truncated)"
	}
	return text
}

func (c *ContextCompactor) budgetToolResults(history *[]Msg) {
	total := 0
	for _, message := range *history {
		for _, block := range message.Blocks {
			if block.Type == BlockToolResult {
				total += len(block.Content)
			}
		}
	}
	if total <= c.toolResultBudget {
		return
	}
	for i := range *history {
		message := &(*history)[i]
		for j := range message.Blocks {
			block := &message.Blocks[j]
			if block.Type != BlockToolResult || len(block.Content) <= 2000 ||
				strings.HasPrefix(block.Content, "[archived]") {
				continue
			}
			file := c.archive(block.ToolUseID, block.Content)
			preview := headTail(block.Content, 1500, 500) + "\n[archived] full output: " + file
			total -= len(block.Content) - len(preview)
			block.Content = preview
			if total <= c.toolResultBudget {
				return
			}
		}
	}
}

func (c *ContextCompactor) truncateToolResults(history *[]Msg, threshold, keep int) {
	for i := range *history {
		message := &(*history)[i]
		for j := range message.Blocks {
			block := &message.Blocks[j]
			if block.Type != BlockToolResult || len(block.Content) <= threshold ||
				strings.HasPrefix(block.Content, "[archived]") {
				continue
			}
			block.Content = headTail(block.Content, keep, keep/2) +
				"\n... (older tool output shortened)"
		}
	}
}

func (c *ContextCompactor) summarize(history *[]Msg, client Client) error {
	keep := c.keepMessages
	if keep > len(*history) {
		keep = len(*history)
	}
	cut := len(*history) - keep
	for cut < len(*history) && !isPlainUserMessage((*history)[cut]) {
		cut++
	}
	if cut <= 0 || cut >= len(*history) {
		return nil
	}
	older := (*history)[:cut]
	prompt := `Summarize the conversation below so another turn can continue the work.
Keep: the user's goal, decisions, files and commands that mattered, concrete
results, and unfinished tasks. Drop small talk and redundant tool output.
Write a compact handoff note, not an essay.

Conversation:
` + Transcript(older, 80_000)

	response, err := client.Complete(
		"You compress conversation history for a coding agent. Be accurate and concise.",
		[]Msg{User(prompt)}, nil)
	if err != nil {
		return err
	}
	summary := strings.TrimSpace(response.Text())
	if summary == "" {
		return nil
	}
	next := []Msg{Assistant([]Block{TextBlock("[Earlier conversation summary]\n" + summary)})}
	next = append(next, (*history)[cut:]...)
	*history = next
	c.compactions++
	fmt.Printf("[compact] summarized %d messages; kept %d\n", cut, len(next)-1)
	return nil
}

func isPlainUserMessage(message Msg) bool {
	if message.Role != "user" {
		return false
	}
	for _, block := range message.Blocks {
		if block.Type == BlockToolResult {
			return false
		}
	}
	return true
}

func (c *ContextCompactor) archive(toolUseID, content string) string {
	if err := os.MkdirAll(c.archiveDir, 0o755); err != nil {
		return "(archive failed: " + err.Error() + ")"
	}
	safe := toolUseID
	if safe == "" {
		safe = fmt.Sprintf("archive-%d", len(content))
	}
	safe = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			return r
		default:
			return '_'
		}
	}, safe)
	file := filepath.Join(c.archiveDir, safe+".txt")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		return "(archive failed: " + err.Error() + ")"
	}
	return file
}

func headTail(text string, head, tail int) string {
	if len(text) <= head+tail+40 {
		return text
	}
	tailStart := len(text) - tail
	for tailStart < len(text) && !utf8.RuneStart(text[tailStart]) {
		tailStart++
	}
	return Truncate(text, head) + fmt.Sprintf("\n... (%d chars omitted) ...\n", len(text)-head-tail) + text[tailStart:]
}
