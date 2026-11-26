package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing

	// RemainingCrossings contains only crossings delivered after the capture.
	RemainingCrossings []Crossing `json:"RemainingCrossings,omitempty"`
	Usage              Usage

	// Continuation metadata. When a valid Recording is supplied, Resident is
	// the engine-confirmed set from which Frames[0].Operations can execute.
	Resident          []string        `json:"Resident,omitempty"`
	StartResident     []string        `json:"StartResident,omitempty"`
	PausedResident    []string        `json:"PausedResident,omitempty"`
	ConfirmedResident []string        `json:"ConfirmedResident,omitempty"`
	StartResidency    []string        `json:"StartResidency,omitempty"`
	Resume            *ResumeMetadata `json:"Resume,omitempty"`
	Checkpoint        *ResumeMetadata `json:"Checkpoint,omitempty"`

	NextFrame   int `json:"NextFrame,omitempty"`
	StartFrame  int `json:"StartFrame,omitempty"`
	ResumeFrame int `json:"ResumeFrame,omitempty"`
	FromFrame   int `json:"FromFrame,omitempty"`
	PauseFrame  int `json:"PauseFrame,omitempty"`
	Start       int `json:"Start,omitempty"`
	ResumeAt    int `json:"ResumeAt,omitempty"`
	NextIndex   int `json:"NextIndex,omitempty"`
	StartIndex  int `json:"StartIndex,omitempty"`
	LiveIndex   int `json:"LiveIndex,omitempty"`

	NextRoute  int `json:"NextRoute,omitempty"`
	StartStep  int `json:"StartStep,omitempty"`
	RouteIndex int `json:"RouteIndex,omitempty"`
	NextStep   int `json:"NextStep,omitempty"`

	RecordedFrames int   `json:"RecordedFrames,omitempty"`
	StartBytes     int64 `json:"StartBytes,omitempty"`
	PauseBytes     int64 `json:"PauseBytes,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
