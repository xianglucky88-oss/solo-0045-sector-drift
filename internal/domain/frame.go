package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision
	Layout     []Placement `json:",omitempty"`
}

type Placement struct {
	Asset  string
	Offset int64
	Size   int64
}
