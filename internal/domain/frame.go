package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// RouteIndex is the original authored route position. Index is the
	// continuous engine frame number, including captured wait frames.
	RouteIndex int `json:"RouteIndex,omitempty"`
	Step       int `json:"Step,omitempty"`
	Route      int `json:"Route,omitempty"`

	// LiveIndex aliases Index for callers that prefer an explicit name.
	LiveIndex  int `json:"LiveIndex,omitempty"`
	FrameIndex int `json:"FrameIndex,omitempty"`
}
