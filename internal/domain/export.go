package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Adaptive-quality ("vista") additions. All omitempty so ordinary plans
	// keep their original document shape.
	Mode        string          `json:"Mode,omitempty"`
	Quality     *QualityReport  `json:"Quality,omitempty"`
	QualityPlan *QualityReport  `json:"QualityPlan,omitempty"`
	TierPlan    *QualityReport  `json:"TierPlan,omitempty"`
	GearPlan    *QualityReport  `json:"GearPlan,omitempty"`
	Vista       *QualityReport  `json:"Vista,omitempty"`
	ShotPlan    *QualityReport  `json:"ShotPlan,omitempty"`
	Shot        *QualityReport  `json:"Shot,omitempty"`
	Adaptive    *QualityReport  `json:"Adaptive,omitempty"`
	Schedule    *QualityReport  `json:"Schedule,omitempty"`
	Result      *QualityReport  `json:"Result,omitempty"`
	Output      *QualityReport  `json:"Output,omitempty"`
	Tiers       []TierReport    `json:"Tiers,omitempty"`
	Gears       []TierReport    `json:"Gears,omitempty"`
	Qualities   []TierReport    `json:"Qualities,omitempty"`
	Levels      []TierReport    `json:"Levels,omitempty"`
	Handoffs    []HandoffRecord `json:"Handoffs,omitempty"`
	Transitions []HandoffRecord `json:"Transitions,omitempty"`
	Switches    []HandoffRecord `json:"Switches,omitempty"`
	GearChanges []HandoffRecord `json:"GearChanges,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
