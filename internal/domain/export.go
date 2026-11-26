package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Route-search outputs. All omitempty so explicitly authored plans keep
	// their original byte representation. Several equivalent keys are emitted
	// so downstream tooling can reconstruct the chosen signposts and moves.
	Signposts          []string    `json:"Signposts,omitempty"`
	Waypoints          []string    `json:"Waypoints,omitempty"`
	RouteSignposts     []string    `json:"RouteSignposts,omitempty"`
	SelectedRoute      []string    `json:"SelectedRoute,omitempty"`
	ChosenRoute        []string    `json:"ChosenRoute,omitempty"`
	SignpostRoute      []string    `json:"SignpostRoute,omitempty"`
	Path               []string    `json:"Path,omitempty"`
	RouteIDs           []string    `json:"RouteIDs,omitempty"`
	Route              []string    `json:"Route,omitempty"`
	SelectedSignposts  []Signpost  `json:"SelectedSignposts,omitempty"`
	SignpostDetails    []Signpost  `json:"SignpostDetails,omitempty"`
	VisitedSignposts   []Signpost  `json:"VisitedSignposts,omitempty"`
	ChosenSignposts    []Signpost  `json:"ChosenSignposts,omitempty"`
	RouteVisits        []Visit     `json:"RouteVisits,omitempty"`
	Visits             []Visit     `json:"Visits,omitempty"`
	Steps              []RouteStep `json:"Steps,omitempty"`
	RouteMoves         []RouteStep `json:"RouteMoves,omitempty"`
	UsedMoves          []RouteStep `json:"UsedMoves,omitempty"`
	MoveLog            []RouteStep `json:"MoveLog,omitempty"`
	LoadedBytes        int64       `json:"LoadedBytes,omitempty"`
	TotalLoadedBytes   int64       `json:"TotalLoadedBytes,omitempty"`
	GrossLoadedBytes   int64       `json:"GrossLoadedBytes,omitempty"`
	SelectedMovesCount int         `json:"SelectedMovesCount,omitempty"`
}

// RouteStep records one actually traversed directed move.
type RouteStep struct {
	Step       int    `json:"Step"`
	From       string `json:"From"`
	To         string `json:"To"`
	Gate       string `json:"Gate,omitempty"`
	Crossing   bool   `json:"Crossing"`
	FromSector string `json:"FromSector,omitempty"`
	ToSector   string `json:"ToSector,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
