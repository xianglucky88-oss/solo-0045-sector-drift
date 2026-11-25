package domain

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy
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
