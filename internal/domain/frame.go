package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// Route-search provenance. Empty for explicitly authored routes, so the
	// legacy wire representation is unchanged.
	ID          string `json:"ID,omitempty"`
	Signpost    string `json:"Signpost,omitempty"`
	SignpostID  string `json:"SignpostID,omitempty"`
	SignpostRef string `json:"SignpostRef,omitempty"`
	Waypoint    string `json:"Waypoint,omitempty"`
	Marker      string `json:"Marker,omitempty"`
	Name        string `json:"Name,omitempty"`
	Ref         string `json:"Ref,omitempty"`
	Node        string `json:"Node,omitempty"`
}
