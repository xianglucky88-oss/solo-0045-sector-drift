package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// Dual-camera extensions. All omitted in legacy single-camera plans so
	// the original artifact bytes stay unchanged.
	Visits  []LocatedVisit `json:"Visits,omitempty"`
	Cameras []CameraFrame  `json:"Cameras,omitempty"`
	Shots   []CameraFrame  `json:"Shots,omitempty"`
}

// LocatedVisit is a Visit tagged with its camera identity.
type LocatedVisit struct {
	Camera   string `json:"Camera,omitempty"`
	Shot     string `json:"Shot,omitempty"`
	ID       string `json:"ID,omitempty"`
	CameraID string `json:"CameraID,omitempty"`
	ShotID   string `json:"ShotID,omitempty"`

	Sector   string
	Position int

	Present bool `json:"Present,omitempty"`
}

// CameraFrame is one camera's per-frame fairness/readiness state.
type CameraFrame struct {
	ID       string `json:"ID,omitempty"`
	Camera   string `json:"Camera,omitempty"`
	Shot     string `json:"Shot,omitempty"`
	CameraID string `json:"CameraID,omitempty"`
	ShotID   string `json:"ShotID,omitempty"`

	Sector   string `json:"Sector,omitempty"`
	Position int    `json:"Position"`
	Present  bool   `json:"Present,omitempty"`
	Active   bool   `json:"Active,omitempty"`

	// Visit mirrors Sector/Position as a nested object for schemas that
	// address frame.Cameras[i].Visit.
	Visit Visit `json:"Visit"`

	// Waiting/Wait/Waited all report the accumulated present-frame wait
	// after this frame's reset/increment. WaitingBefore is the priority
	// value used before this frame's readiness reset.
	Waiting       int `json:"Waiting"`
	Wait          int `json:"Wait"`
	Waited        int `json:"Waited"`
	WaitingFrames int `json:"WaitingFrames"`
	WaitFrames    int `json:"WaitFrames"`
	WaitingBefore int `json:"WaitingBefore"`

	Ready             bool        `json:"Ready"`
	HasReadyCandidate bool        `json:"HasReadyCandidate"`
	ReadyCandidates   []string    `json:"ReadyCandidates,omitempty"`
	NearCandidates    []Candidate `json:"-"`
	Decisions         []Decision  `json:"-"`
}
