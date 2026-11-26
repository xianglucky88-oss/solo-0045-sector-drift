package domain

import "encoding/json"

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// raw keeps the original JSON object so the adaptive-quality planner can
	// accept the many field spellings used by quality-segment requests. It is
	// ignored by encoding/json and by positional-free struct literals.
	raw map[string]json.RawMessage
}
type Sector struct {
	ID     string
	Assets []string
}
type Asset struct {
	ID    string
	Bytes int64
	After []string
}
type Gate struct {
	ID, From, To string
	At, Reach    int
	OneWay       bool
}
type Visit struct {
	Sector   string
	Position int
}
type Policy struct {
	Budget            int64
	Lookahead, Retain int
	Pinned            []string
}
