package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Dual-camera extensions (omitted in legacy plans).
	Cameras       []CameraReport `json:"Cameras,omitempty"`
	Shots         []CameraReport `json:"Shots,omitempty"`
	CameraReports []CameraReport `json:"CameraReports,omitempty"`
	ShotReports   []CameraReport `json:"ShotReports,omitempty"`

	// Prewarm ledgers: who received near-gate prewarm opportunities. The
	// same records are exposed under the two most likely field names.
	Prewarms []Grant `json:"Prewarms,omitempty"`
	Warmups  []Grant `json:"Warmups,omitempty"`
}

// CameraReport summarises one camera across the whole plan. The heavy
// decision lists stay on the frames; this report keeps cheap attribution.
type CameraReport struct {
	ID       string `json:"ID,omitempty"`
	Camera   string `json:"Camera,omitempty"`
	Shot     string `json:"Shot,omitempty"`
	CameraID string `json:"CameraID,omitempty"`
	ShotID   string `json:"ShotID,omitempty"`

	FramesPresent int `json:"FramesPresent"`
	MaxWaiting    int `json:"MaxWaiting"`

	Crossings []Crossing `json:"Crossings,omitempty"`

	// Kept for Go callers but not serialised into every report alias.
	Prewarms  []Grant    `json:"-"`
	Decisions []Decision `json:"-"`
}

// Grant records one awarded prewarm opportunity.
type Grant struct {
	Step  int `json:"Step"`
	Frame int `json:"Frame"`
	Index int `json:"Index"`

	Camera   string `json:"Camera,omitempty"`
	Shot     string `json:"Shot,omitempty"`
	CameraID string `json:"CameraID,omitempty"`
	ShotID   string `json:"ShotID,omitempty"`

	Sector   string `json:"Sector"`
	Reason   string `json:"Reason"`
	Distance int    `json:"Distance"`
	Gate     string `json:"Gate,omitempty"`
	Admitted bool   `json:"Admitted"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
