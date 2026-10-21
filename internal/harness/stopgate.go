package harness

// StopGate is an independent check at the loop's return boundary.
type StopDecision struct {
	Stop   bool
	Reason string
	Status string
}

func StopWith(status string) StopDecision {
	return StopDecision{Stop: true, Status: status}
}

func ContinueWith(reason string) StopDecision {
	return StopDecision{Stop: false, Reason: reason, Status: "working"}
}

type StopGate interface {
	BeforeStop(history []Msg, lastText string) (StopDecision, error)
}
