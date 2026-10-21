package harness

import (
	"fmt"
	"strings"
)

// Goal is an independent evaluator at the loop's stop boundary.
type Goal struct {
	goal           string
	client         Client
	maxEvaluations int
	evaluations    int
}

func NewGoal(goal string, client Client) *Goal {
	return &Goal{
		goal:           strings.TrimSpace(goal),
		client:         client,
		maxEvaluations: GetInt("GOAL_MAX_TURNS", 12),
	}
}

func (g *Goal) Text() string     { return g.goal }
func (g *Goal) Evaluations() int { return g.evaluations }
func (g *Goal) StatusLine() string {
	return fmt.Sprintf("goal=%s evaluations=%d/%d", g.goal, g.evaluations, g.maxEvaluations)
}

func (g *Goal) BeforeStop(history []Msg, lastText string) (StopDecision, error) {
	if g.goal == "" {
		return StopWith("completed"), nil
	}
	g.evaluations++
	if g.evaluations > g.maxEvaluations {
		return StopWith("max_turns"), nil
	}

	prompt := fmt.Sprintf(`Goal:
%s

Conversation so far:
%s

Decide whether the goal is complete. Use only concrete evidence in the
conversation. Do not assume that an unreported command succeeded or that a
planned step was executed. If the goal can no longer be completed, set
impossible to true.

Return JSON only:
{"ok": true|false, "impossible": true|false, "reason": "one short sentence"}`, g.goal, Transcript(history, 60_000))

	response, err := g.client.Complete(
		"You are an independent goal evaluator. You have no tools. Judge the conversation, not the worker's intentions.",
		[]Msg{User(prompt)}, nil)
	if err != nil {
		return StopDecision{}, err
	}
	parsed := extractObject(response.Text())
	if parsed == nil {
		return ContinueWith("The goal evaluator returned an unreadable decision. " +
			"Continue working and report concrete verification results."), nil
	}
	if Bool(parsed, "ok", false) {
		return StopWith("goal_met"), nil
	}
	if Bool(parsed, "impossible", false) {
		return StopWith("impossible"), nil
	}
	reason := strings.TrimSpace(Str(parsed, "reason"))
	if reason == "" {
		reason = "The goal is not met yet."
	}
	return ContinueWith("Goal check: " + reason +
		"\nKeep working. Run the verification command and report its concrete result."), nil
}

func extractObject(text string) map[string]any {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil
	}
	parsed, err := ParseJSON(text[start : end+1])
	if err != nil {
		return nil
	}
	return AsObject(parsed)
}
