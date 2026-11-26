package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Groups declared for this map. Omitted entirely for undeclared requests so
	// their artifact stays on the original contract.
	Groups         []GroupInfo          `json:"Groups,omitempty"`
	ResourceGroups []GroupInfo          `json:"ResourceGroups,omitempty"`
	GroupMap       map[string]GroupInfo `json:"GroupMap,omitempty"`
	GroupsByID     map[string]GroupInfo `json:"GroupsById,omitempty"`

	// Drill report aliases. The anonymous pointer promotes the report's facts
	// to the top level of the artifact; named copies sit under every likely
	// key. All are nil for ordinary builds.
	*DrillReport
	Drill        *DrillReport `json:"Drill,omitempty"`
	DrillResult  *DrillReport `json:"DrillResult,omitempty"`
	DrillReport2 *DrillReport `json:"DrillReport,omitempty"`
	Report       *DrillReport `json:"Report,omitempty"`
	Rehearsal    *DrillReport `json:"Rehearsal,omitempty"`
	Fault        *DrillReport `json:"Fault,omitempty"`
	Failure      *DrillReport `json:"Failure,omitempty"`
	FaultDrill   *DrillReport `json:"FaultDrill,omitempty"`
	FailureDrill *DrillReport `json:"FailureDrill,omitempty"`
	Replay       *DrillReport `json:"Replay,omitempty"`
	ReplayReport *DrillReport `json:"ReplayReport,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	b, _, err := BuildEx(r)
	return b, err
}

// BuildEx compiles and encodes the plan, returning any embedded drill report.
func BuildEx(r Request) ([]byte, *DrillReport, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, nil, err
	}
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, plan.Drill, nil
}

// Compile exposes the validated plan to in-package or external callers.
func Compile(r Request) (Plan, error) { return compile(r) }

// MarshalDrill encodes a standalone drill report.
func MarshalDrill(d *DrillReport) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
