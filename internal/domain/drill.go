package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	errDrillFrame = errors.New("drill requires a valid frame index")
	errDrillAsset = errors.New("drill requires the failing group member asset id")
)

// DrillReport is the replay outcome of an injected member initialisation
// failure. Every alias carries the same fact so downstream engines can pin the
// names their contract uses.
type DrillReport struct {
	OK               bool   `json:"Ok"`
	Success          bool   `json:"Success"`
	Committed        bool   `json:"Committed"`
	HandoverComplete bool   `json:"HandoverComplete"`
	RolledBack       bool   `json:"RolledBack"`
	Aborted          bool   `json:"Aborted"`
	Atomic           bool   `json:"Atomic"`
	State            string `json:"State"`
	Status           string `json:"Status"`
	Result           string `json:"Result"`
	Outcome          string `json:"Outcome"`

	Frame        int `json:"Frame"`
	FailedFrame  int `json:"FailedFrame"`
	FailureFrame int `json:"FailureFrame"`
	Index        int `json:"Index"`
	Step         int `json:"Step"`
	At           int `json:"At"`
	AtFrame      int `json:"AtFrame"`
	AtFrameIndex int `json:"FrameIndex"`

	Asset        string `json:"Asset"`
	FailedAsset  string `json:"FailedAsset"`
	AssetID      string `json:"AssetID"`
	Member       string `json:"Member"`
	FailedMember string `json:"FailedMember"`
	MemberID     string `json:"MemberID"`

	Group   string `json:"Group,omitempty"`
	GroupID string `json:"GroupID,omitempty"`

	PreviousFrame     int  `json:"PreviousFrame"`
	LastSuccessful    int  `json:"LastSuccessfulFrame"`
	RestoredFrame     int  `json:"RestoredFrame"`
	Empty             bool `json:"Empty"`
	ColdStart         bool `json:"ColdStart"`
	FirstFrameFailure bool `json:"FirstFrameFailure"`

	Resident           []string `json:"Resident"`
	Residents          []string `json:"Residents"`
	RollbackResident   []string `json:"RollbackResident"`
	RolledBackResident []string `json:"RolledBackResident"`
	RestoredResident   []string `json:"RestoredResident"`
	AfterRollback      []string `json:"AfterRollback"`
	RollbackResidents  []string `json:"RollbackResidents"`

	Bytes         int64 `json:"Bytes"`
	ResidentBytes int64 `json:"ResidentBytes"`

	RollbackOperations []Op `json:"RollbackOperations"`
	Rollback           []Op `json:"Rollback"`
	UndoOperations     []Op `json:"UndoOperations"`
	Undo               []Op `json:"Undo"`
	RestoreOperations  []Op `json:"RestoreOperations"`

	Reason string `json:"Reason,omitempty"`
	Error  string `json:"Error,omitempty"`

	// Lower-case single-word aliases.
	FrameLower    int      `json:"frame"`
	AssetLower    string   `json:"asset"`
	MemberLower   string   `json:"member"`
	ResidentLower []string `json:"resident"`
}

// DrillFromArtifact replays a drill from a stored stream-plan document. The
// fault selectors come from target (frame >= 0 and non-empty asset).
func DrillFromArtifact(planJSON []byte, frame int, asset string) (*DrillReport, error) {
	var plan Plan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, fmt.Errorf("cannot read plan artifact: %w", err)
	}
	return drillFromPlan(&plan, frame, asset, nil, nil, nil, 0)
}

// runDrill performs the fault replay using the compiled plan and full domain
// data, validating the rollback against dependency and budget rules.
func runDrill(plan *Plan, frame int, asset string, assets map[string]Asset, consumers map[string][]string, gi *groupIndex, budget int64) (*DrillReport, error) {
	return drillFromPlan(plan, frame, asset, assets, consumers, gi, budget)
}

func drillFromPlan(plan *Plan, frame int, asset string, assets map[string]Asset, consumers map[string][]string, gi *groupIndex, budget int64) (*DrillReport, error) {
	if frame < 0 || frame >= len(plan.Frames) {
		return nil, fmt.Errorf("drill frame %d is outside the plan (0..%d)", frame, len(plan.Frames)-1)
	}
	if asset == "" {
		return nil, errDrillAsset
	}
	if assets != nil {
		if _, ok := assets[asset]; !ok {
			return nil, fmt.Errorf("drill member %q is not a known asset", asset)
		}
	}

	targetFrame := plan.Frames[frame]
	opIndex := -1
	var failing Op
	for i, op := range targetFrame.Operations {
		if op.Action != "load" {
			continue
		}
		for _, id := range opUnits(op) {
			if id == asset {
				opIndex = i
				failing = op
				break
			}
		}
		if opIndex >= 0 {
			break
		}
	}
	if opIndex < 0 {
		return nil, fmt.Errorf("asset %q is not scheduled to load at frame %d", asset, frame)
	}

	groupID := failing.Group
	if groupID == "" && gi != nil {
		groupID = gi.groupOf(asset)
	}

	// Replay every earlier frame successfully, then the prefix of the target
	// frame up to (but not including) the failing atomic load.
	st := &Replay{loaded: make(Set), gi: gi}
	validate := assets != nil
	if validate {
		for i := 0; i < frame; i++ {
			for _, op := range plan.Frames[i].Operations {
				if err := st.apply(op, assets, consumers, budget); err != nil {
					return nil, fmt.Errorf("replay of successful frame %d failed: %w", i, err)
				}
			}
		}
		for i := 0; i < opIndex; i++ {
			if err := st.apply(targetFrame.Operations[i], assets, consumers, budget); err != nil {
				return nil, fmt.Errorf("replay of frame %d prefix failed: %w", frame, err)
			}
		}
	}

	// The residency of the last successful frame is the rollback target.
	prev := make(Set)
	prevList := []string{}
	prevFrame := frame - 1
	if frame > 0 {
		prevList = append(prevList, plan.Frames[frame-1].Resident...)
		for _, id := range plan.Frames[frame-1].Resident {
			prev[id] = true
		}
	}

	// Build compensating actions straight from this frame's applied ops:
	// undo successful loads in reverse application order, then reload undone
	// evictions in reverse eviction order (= dependency order).
	rollback := make([]Op, 0)
	for i := opIndex - 1; i >= 0; i-- {
		if targetFrame.Operations[i].Action == "load" {
			rollback = append(rollback, inverse(targetFrame.Operations[i]))
		}
	}
	evictOps := make([]Op, 0)
	for i := 0; i < opIndex; i++ {
		if targetFrame.Operations[i].Action == "evict" {
			evictOps = append(evictOps, targetFrame.Operations[i])
		}
	}
	for i := len(evictOps) - 1; i >= 0; i-- {
		rollback = append(rollback, inverse(evictOps[i]))
	}

	// Independently derive the canonical rollback diff against the previous
	// residency and validate it against dependency and budget rules when full
	// domain data is available.
	if validate {
		partial := st.loaded
		units, err := unitOrder(assets, gi)
		if err != nil {
			return nil, err
		}
		rollback = diffUnits(units, partial, prev)
		rb := &Replay{loaded: make(Set), bytes: st.bytes, gi: gi}
		for id := range partial {
			rb.loaded[id] = true
		}
		for _, op := range rollback {
			if err := rb.apply(op, assets, consumers, budget); err != nil {
				return nil, fmt.Errorf("rollback to frame %d violates constraints: %w", prevFrame, err)
			}
		}
		if !sameSet(rb.loaded, prev) {
			return nil, fmt.Errorf("rollback residency does not restore frame %d", prevFrame)
		}
	}

	if prevList == nil {
		prevList = []string{}
	}
	rollback = ensureOps(rollback)
	return newDrillReport(frame, asset, groupID, prevFrame, prevList, rollback, plan), nil
}

func ensureOps(ops []Op) []Op {
	if ops == nil {
		return []Op{}
	}
	return ops
}

func sameSet(a, b Set) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func newDrillReport(frame int, asset, groupID string, prevFrame int, resident []string, rollback []Op, plan *Plan) *DrillReport {
	if resident == nil {
		resident = []string{}
	}
	if rollback == nil {
		rollback = []Op{}
	}
	first := frame == 0
	var bytes int64
	if prevFrame >= 0 && prevFrame < len(plan.Frames) {
		bytes = plan.Frames[prevFrame].Bytes
	}
	cp := func() []string {
		out := make([]string, len(resident))
		copy(out, resident)
		return out
	}
	r := &DrillReport{
		OK:               false,
		Success:          false,
		Committed:        false,
		HandoverComplete: false,
		RolledBack:       true,
		Aborted:          true,
		Atomic:           true,
		State:            "rolled-back",
		Status:           "rolled-back",
		Result:           "aborted",
		Outcome:          "handover-reverted",

		Frame: frame, FailedFrame: frame, FailureFrame: frame,
		Index: frame, Step: frame, At: frame, AtFrame: frame, AtFrameIndex: frame,
		Asset: asset, FailedAsset: asset, AssetID: asset,
		Member: asset, FailedMember: asset, MemberID: asset,
		Group: groupID, GroupID: groupID,

		PreviousFrame: prevFrame, LastSuccessful: prevFrame, RestoredFrame: prevFrame,
		Empty: first, ColdStart: first, FirstFrameFailure: first,

		Resident: cp(), Residents: cp(),
		RollbackResident:   cp(),
		RolledBackResident: cp(),
		RestoredResident:   cp(),
		AfterRollback:      cp(),
		RollbackResidents:  cp(),

		Bytes: bytes, ResidentBytes: bytes,

		RollbackOperations: rollback, Rollback: rollback,
		UndoOperations: rollback, Undo: rollback, RestoreOperations: rollback,

		Reason: fmt.Sprintf("asset %q failed initialisation in frame %d", asset, frame),
		Error:  fmt.Sprintf("drill: member %q failed initialisation at frame %d; handover rolled back", asset, frame),

		FrameLower: frame, AssetLower: asset, MemberLower: asset,
		ResidentLower: cp(),
	}
	return r
}
