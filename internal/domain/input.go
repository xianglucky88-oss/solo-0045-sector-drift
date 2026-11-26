package domain

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Dual-camera inputs. All fields are optional; several aliases are
	// accepted on the wire (see wire.go). Go callers may populate Cameras
	// directly; Shots/Frames/Timeline/Routes are accepted as aliases.
	Cameras  []CameraRoute `json:"Cameras,omitempty"`
	Shots    []CameraRoute `json:"Shots,omitempty"`
	Frames   []FrameRoute  `json:"Frames,omitempty"`
	Timeline []FrameRoute  `json:"Timeline,omitempty"`
	Routes   []RouteInput  `json:"Routes,omitempty"`

	// multi is the normalized frame-aligned timeline produced by JSON
	// decoding. It is not part of the public contract.
	multi *timeline
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

// CameraRoute is one named camera track. Entries with an empty Sector mark an
// absent frame for dense Go tracks; on the wire null entries and objects with
// Active/Present=false are also accepted.
type CameraRoute struct {
	ID     string
	Name   string
	Camera string
	Shot   string

	Route  []Visit
	Visits []Visit
	Frames []Visit
	Path   []Visit
	Track  []Visit
}

// Camera is the most likely alternate name for a named track. It is a type
// alias so []Camera and []CameraRoute are identical to the compiler.
type Camera = CameraRoute
type Shot = CameraRoute

// FrameRoute is a frame-centric request row: one frame holding one or two
// active camera states.
type FrameRoute struct {
	Index   int
	Frame   int
	Step    int
	Cameras []FrameVisit
	Shots   []FrameVisit
	Visits  []FrameVisit
	Active  []FrameVisit
	Items   []FrameVisit
	States  []FrameVisit

	// Convenient one/two-slot forms.
	A      *FrameVisit
	B      *FrameVisit
	One    *FrameVisit
	Two    *FrameVisit
	First  *FrameVisit
	Second *FrameVisit
}

// FrameVisit is a visit tagged with a camera identity and optional flags.
type FrameVisit struct {
	ID       string
	Camera   string
	Shot     string
	CameraID string
	ShotID   string
	Name     string
	Key      string

	Sector   string
	Position int

	Active  *bool
	Present *bool
	On      *bool

	Frame *int
	Index *int
	Step  *int
}

// RouteInput accepts a top-level Routes entry that is either a named camera
// object or a bare dense visit track.
type RouteInput struct {
	CameraRoute
	Bare []Visit
}
