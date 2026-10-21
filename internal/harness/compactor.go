package harness

type Compactor interface {
	BeforeRequest(history *[]Msg, client Client) error
}

type NoCompactor struct{}

func (NoCompactor) BeforeRequest(*[]Msg, Client) error { return nil }
