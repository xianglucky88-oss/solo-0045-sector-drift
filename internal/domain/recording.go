package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Recording is the verified engine capture through the pause frame.
type Recording struct {
	Frames          []RecordedFrame `json:"Frames,omitempty"`
	Steps           []RecordedFrame `json:"Steps,omitempty"`
	Records         []RecordedFrame `json:"Records,omitempty"`
	Entries         []RecordedFrame `json:"Entries,omitempty"`
	RecordedFrames  []RecordedFrame `json:"RecordedFrames,omitempty"`
	LiveFrames      []RecordedFrame `json:"LiveFrames,omitempty"`
	ActualFrames    []RecordedFrame `json:"ActualFrames,omitempty"`
	TimelineFrames  []RecordedFrame `json:"TimelineFrames,omitempty"`
	TraceFrames     []RecordedFrame `json:"TraceFrames,omitempty"`
	FrameRecords    []RecordedFrame `json:"FrameRecords,omitempty"`
	RecordingFrames []RecordedFrame `json:"RecordingFrames,omitempty"`
	CapturedFrames  []RecordedFrame `json:"CapturedFrames,omitempty"`
	HistoryFrames   []RecordedFrame `json:"HistoryFrames,omitempty"`
	Timeline        []RecordedFrame `json:"Timeline,omitempty"`
	Points          []RecordedFrame `json:"Points,omitempty"`
	Events          []RecordedFrame `json:"Events,omitempty"`
	Samples         []RecordedFrame `json:"Samples,omitempty"`
	Items           []RecordedFrame `json:"Items,omitempty"`
	Log             []RecordedFrame `json:"Log,omitempty"`

	Resident          []string `json:"Resident,omitempty"`
	Residency         []string `json:"Residency,omitempty"`
	ResidentSet       []string `json:"ResidentSet,omitempty"`
	Residents         []string `json:"Residents,omitempty"`
	Paused            []string `json:"Paused,omitempty"`
	PauseSet          []string `json:"PauseSet,omitempty"`
	PausedSet         []string `json:"PausedSet,omitempty"`
	PauseResidency    []string `json:"PauseResidency,omitempty"`
	PausedResident    []string `json:"PausedResident,omitempty"`
	PauseResident     []string `json:"PauseResident,omitempty"`
	PausedResidents   []string `json:"PausedResidents,omitempty"`
	Confirmed         []string `json:"Confirmed,omitempty"`
	ConfirmedSet      []string `json:"ConfirmedSet,omitempty"`
	ConfirmedResident []string `json:"ConfirmedResident,omitempty"`
	EngineResident    []string `json:"EngineResident,omitempty"`
	EngineResidency   []string `json:"EngineResidency,omitempty"`
	FinalResident     []string `json:"FinalResident,omitempty"`
	EndResident       []string `json:"EndResident,omitempty"`
}

// RecordedFrame is one continuous engine frame. A player may remain on the
// same authored Route entry for several captured frames.
type RecordedFrame struct {
	Sector   string `json:"Sector,omitempty"`
	Position int    `json:"Position,omitempty"`

	Visit   Visit `json:"Visit,omitempty"`
	At      int   `json:"At,omitempty"`
	Arrival Visit `json:"Arrival,omitempty"`
	AtVisit Visit `json:"AtVisit,omitempty"`
	RouteAt Visit `json:"RouteAt,omitempty"`
	Reached Visit `json:"Reached,omitempty"`

	Step          int `json:"Step,omitempty"`
	Index         int `json:"Index,omitempty"`
	Frame         int `json:"Frame,omitempty"`
	FrameIndex    int `json:"FrameIndex,omitempty"`
	LiveIndex     int `json:"LiveIndex,omitempty"`
	RouteIndex    int `json:"RouteIndex,omitempty"`
	RouteStep     int `json:"RouteStep,omitempty"`
	Route         int `json:"Route,omitempty"`
	RoutePosition int `json:"RoutePosition,omitempty"`
	AtRoute       int `json:"AtRoute,omitempty"`
	RouteAtIndex  int `json:"RouteAtIndex,omitempty"`
	ReachedIndex  int `json:"ReachedIndex,omitempty"`
	ReachedStep   int `json:"ReachedStep,omitempty"`
	OriginalIndex int `json:"OriginalIndex,omitempty"`
	AuthoredIndex int `json:"AuthoredIndex,omitempty"`
	Waypoint      int `json:"Waypoint,omitempty"`
	WaypointIndex int `json:"WaypointIndex,omitempty"`
	PositionIndex int `json:"PositionIndex,omitempty"`
	VisitIndex    int `json:"VisitIndex,omitempty"`
	RouteVisit    int `json:"RouteVisit,omitempty"`
	StepIndex     int `json:"StepIndex,omitempty"`
	ArrivedIndex  int `json:"ArrivedIndex,omitempty"`
	ArrivalIndex  int `json:"ArrivalIndex,omitempty"`
	AtIndex       int `json:"AtIndex,omitempty"`

	Operations []Op `json:"Operations,omitempty"`
	Ops        []Op `json:"Ops,omitempty"`
	Actions    []Op `json:"Actions,omitempty"`

	Loads          []string `json:"Loads,omitempty"`
	Unloads        []string `json:"Unloads,omitempty"`
	LoadAssets     []string `json:"LoadAssets,omitempty"`
	UnloadAssets   []string `json:"UnloadAssets,omitempty"`
	LoadedAssets   []string `json:"LoadedAssets,omitempty"`
	UnloadedAssets []string `json:"UnloadedAssets,omitempty"`

	Resident     []string   `json:"Resident,omitempty"`
	Residents    []string   `json:"Residents,omitempty"`
	EndResident  []string   `json:"EndResident,omitempty"`
	EndResidency []string   `json:"EndResidency,omitempty"`
	Required     []string   `json:"Required,omitempty"`
	Bytes        int64      `json:"Bytes,omitempty"`
	Decisions    []Decision `json:"Decisions,omitempty"`

	atObject    map[string]any `json:"-"`
	routeObject map[string]any `json:"-"`
}

// UnmarshalJSON allows At/Route to be either a numeric route index or a full
// Visit object while retaining the integer fields for Go callers.
func (f *RecordedFrame) UnmarshalJSON(data []byte) error {
	type plain RecordedFrame
	var aux struct {
		plain
		At    json.RawMessage `json:"At"`
		Route json.RawMessage `json:"Route"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&aux); err != nil {
		return err
	}
	*f = RecordedFrame(aux.plain)
	if len(aux.At) > 0 && string(bytes.TrimSpace(aux.At)) != "null" {
		var n int
		if err := json.Unmarshal(aux.At, &n); err == nil {
			f.At = n
		} else {
			var v Visit
			if err := json.Unmarshal(aux.At, &v); err == nil && v.Sector != "" {
				f.AtVisit = v
			} else {
				_ = json.Unmarshal(aux.At, &f.atObject)
			}
		}
	}
	if len(aux.Route) > 0 && string(bytes.TrimSpace(aux.Route)) != "null" {
		var n int
		if err := json.Unmarshal(aux.Route, &n); err == nil {
			f.Route = n
		} else {
			var v Visit
			if err := json.Unmarshal(aux.Route, &v); err == nil && v.Sector != "" {
				f.RouteAt = v
			} else {
				_ = json.Unmarshal(aux.Route, &f.routeObject)
			}
		}
	}
	return nil
}

// Source-compatible type aliases for alternate naming in tests.
type (
	Record           = Recording
	Live             = Recording
	Actual           = Recording
	History          = Recording
	Session          = Recording
	Capture          = Recording
	SessionRecord    = Recording
	StreamRecord     = Recording
	LiveCapture      = Recording
	EngineRecording  = Recording
	ActualTrace      = Recording
	LiveTrace        = Recording
	EngineTrace      = Recording
	RecordLog        = Recording
	LiveRecording    = Recording
	ActualRecording  = Recording
	Trace            = Recording
	Timeline         = Recording
	Checkpoint       = Recording
	Resume           = Recording
	Continuation     = Recording
	RecordedTimeline = Recording

	RecordingFrame = RecordedFrame
	CaptureFrame   = RecordedFrame
	RecordStep     = RecordedFrame
	FrameRecord    = RecordedFrame
	LiveFrame      = RecordedFrame
	ActualFrame    = RecordedFrame
	TraceFrame     = RecordedFrame
	TimelineFrame  = RecordedFrame
	LiveStep       = RecordedFrame
	EngineFrame    = RecordedFrame
	Operation      = Op
	Action         = Op
)

// RecordingError identifies the first invalid captured frame. Frame is
// zero-based; Error also prints the one-based number for operators.
type RecordingError struct {
	Frame int
	Index int
	At    int
	Step  int
	Err   error
}

func (e *RecordingError) Error() string {
	if e == nil || e.Err == nil {
		return fmt.Sprintf("recording frame %d is invalid", e.Frame)
	}
	return fmt.Sprintf("recording frame %d (frames[%d], one-based frame %d): %v", e.Frame, e.Frame, e.Frame+1, e.Err)
}

func (e *RecordingError) Unwrap() error { return e.Err }

type RecordError = RecordingError
type ResumeInfo = ResumeMetadata
type ContinuationInfo = ResumeMetadata
type CheckpointInfo = ResumeMetadata

// ResumeMetadata describes the boundary between an engine capture and the
// delivered continuation.
type ResumeMetadata struct {
	AfterFrame      int      `json:"AfterFrame"`
	NextFrame       int      `json:"NextFrame"`
	PauseFrame      int      `json:"PauseFrame"`
	NextRoute       int      `json:"NextRoute"`
	RouteIndex      int      `json:"RouteIndex"`
	PausedResident  []string `json:"PausedResident"`
	StartResident   []string `json:"StartResident"`
	RecordedFrames  int      `json:"RecordedFrames"`
	RemainingFrames int      `json:"RemainingFrames"`
}

type recordingOutcome struct {
	nextRoute int
	nextLive  int
	replay    Replay
	history   history
	pause     Set
	endSets   []Set
}

func firstSlice[T any](choices ...[]T) []T {
	for _, x := range choices {
		if x != nil {
			return x
		}
	}
	return nil
}

func (r *Request) normalizeRecording() *Recording {
	if r == nil {
		return nil
	}
	candidates := []*Recording{
		r.Recording, r.Replay, r.Capture, r.Record, r.Live, r.LiveRecord, r.LiveRecording,
		r.Actual, r.ActualRecord, r.ActualRecording, r.Trace, r.LiveTrace, r.EngineTrace,
		r.RecordLog, r.Timeline,
		r.History, r.Checkpoint, r.Resume, r.Continuation, r.Session,
		r.ReplayRecord, r.RecordedTimeline, r.ExecutionRecord, r.EngineRecord,
		r.RuntimeRecord, r.PlaybackRecording,
	}
	flatFrames := firstSlice(r.RecordedFrames, r.LiveFrames, r.ActualFrames, r.TraceFrames, r.FrameLog)
	flatResident := firstSlice(r.Resident, r.Residency, r.ResidentSet, r.PauseSet,
		r.PausedSet, r.PausedResident, r.PauseResident, r.PauseResidency,
		r.PausedResidents, r.ConfirmedResident, r.ConfirmedSet,
		r.EngineResident, r.EngineResidency)

	out := &Recording{}
	sawWrapper := false
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		sawWrapper = true
		candidate.collapse()
		if out.Frames == nil && candidate.Frames != nil {
			out.Frames = candidate.Frames
		}
		if out.Resident == nil && candidate.Resident != nil {
			out.Resident = candidate.Resident
		}
	}
	if out.Frames == nil {
		out.Frames = flatFrames
	}
	if out.Resident == nil {
		out.Resident = flatResident
	}
	if !sawWrapper && flatFrames == nil && flatResident == nil {
		return nil
	}
	out.collapse()
	r.Recording = out
	return out
}

func (rec *Recording) collapse() {
	if rec == nil {
		return
	}
	if rec.Frames == nil {
		rec.Frames = firstSlice(rec.Steps, rec.Records, rec.Entries, rec.RecordedFrames,
			rec.LiveFrames, rec.ActualFrames, rec.TimelineFrames, rec.TraceFrames,
			rec.FrameRecords, rec.RecordingFrames, rec.CapturedFrames, rec.HistoryFrames,
			rec.Timeline, rec.Points, rec.Events, rec.Samples, rec.Items, rec.Log)
	}
	if rec.Resident == nil {
		rec.Resident = firstSlice(rec.Residency, rec.ResidentSet, rec.Residents, rec.Paused,
			rec.PauseSet, rec.PausedSet, rec.PauseResidency, rec.PausedResident,
			rec.PauseResident, rec.PausedResidents, rec.Confirmed, rec.ConfirmedSet,
			rec.ConfirmedResident, rec.EngineResident, rec.EngineResidency,
			rec.FinalResident, rec.EndResident)
	}
	for i := range rec.Frames {
		rec.Frames[i].normalize()
		if rec.Frames[i].Resident == nil {
			rec.Frames[i].Resident = firstSlice(rec.Frames[i].Residents, rec.Frames[i].EndResident, rec.Frames[i].EndResidency)
		}
	}
}

func (f *RecordedFrame) normalize() {
	if f == nil {
		return
	}
	if f.Operations == nil {
		f.Operations = firstSlice(f.Ops, f.Actions)
	}
	if f.Operations == nil {
		var ops []Op
		unloads := firstSlice(f.Unloads, f.UnloadAssets, f.UnloadedAssets)
		loads := firstSlice(f.Loads, f.LoadAssets, f.LoadedAssets)
		// Separate lists do not encode cross-list order. Evictions first is the
		// streaming-safe order and matches the compiler's diff ordering.
		for _, id := range unloads {
			ops = append(ops, Op{Action: "evict", Asset: id})
		}
		for _, id := range loads {
			ops = append(ops, Op{Action: "load", Asset: id})
		}
		if ops != nil {
			f.Operations = ops
		}
	}
	if f.Sector == "" {
		switch {
		case f.Visit.Sector != "":
			f.Sector, f.Position = f.Visit.Sector, f.Visit.Position
		case f.Arrival.Sector != "":
			f.Sector, f.Position = f.Arrival.Sector, f.Arrival.Position
		case f.AtVisit.Sector != "":
			f.Sector, f.Position = f.AtVisit.Sector, f.AtVisit.Position
		case f.RouteAt.Sector != "":
			f.Sector, f.Position = f.RouteAt.Sector, f.RouteAt.Position
		case f.Reached.Sector != "":
			f.Sector, f.Position = f.Reached.Sector, f.Reached.Position
		}
	}
}

func (f RecordedFrame) routeHint() (int, bool) {
	values := []int{
		f.RouteIndex, f.RouteStep, f.Route, f.Step, f.StepIndex,
		f.RoutePosition, f.PositionIndex, f.VisitIndex, f.RouteVisit,
		f.ArrivedIndex, f.ArrivalIndex,
		f.OriginalIndex, f.AuthoredIndex, f.ReachedIndex, f.ReachedStep,
		f.AtRoute, f.RouteAtIndex, f.WaypointIndex, f.Waypoint,
		f.Position, f.At, f.Index, f.FrameIndex, f.LiveIndex, f.AtIndex, f.Frame,
	}
	for _, v := range values {
		if v != 0 {
			return v, true
		}
	}
	return 0, false
}

func (f RecordedFrame) targetVisit(route []Visit, frameIndex, previous int) (int, error) {
	allowed := []int{0}
	if frameIndex > 0 {
		allowed = []int{previous}
		if previous+1 < len(route) {
			allowed = append(allowed, previous+1)
		}
	}
	// An explicit Visit is the strongest correspondence.
	if f.Sector != "" {
		for _, idx := range allowed {
			if route[idx].Sector == f.Sector && route[idx].Position == f.Position {
				return idx, nil
			}
		}
		// If only sector was supplied, accept the allowed occurrence when its
		// authored position is unambiguous.
		if f.Position == 0 {
			for _, idx := range allowed {
				if route[idx].Sector == f.Sector {
					return idx, nil
				}
			}
		}
		return 0, fmt.Errorf("arrival %s:%d is not the same or next authored route position", f.Sector, f.Position)
	}
	if idx, ok := visitLikeTarget([]any{f.Visit, f.Arrival, f.AtVisit, f.RouteAt, f.Reached, f.atObject, f.routeObject, f.At, f.Route}, route, allowed); ok {
		return idx, nil
	}
	if hint, ok := f.routeHint(); ok {
		for _, idx := range allowed {
			if hint == idx {
				return idx, nil
			}
		}
		return 0, fmt.Errorf("route position %d is not the same or next authored route position", hint)
	}
	// A zero-valued route reference is indistinguishable from an omitted one
	// in Go, and means "stay at the current/first position" for a continuous
	// capture frame.
	return allowed[0], nil
}

func visitLikeTarget(values []any, route []Visit, allowed []int) (int, bool) {
	for _, value := range values {
		switch at := value.(type) {
		case Visit:
			for _, idx := range allowed {
				if route[idx] == at {
					return idx, true
				}
			}
		case map[string]any:
			sector, _ := at["Sector"].(string)
			if sector == "" {
				sector, _ = at["sector"].(string)
			}
			pos := anyInt(at["Position"], at["position"])
			for _, idx := range allowed {
				if route[idx].Sector == sector && route[idx].Position == pos {
					return idx, true
				}
			}
		}
	}
	return 0, false
}

func anyInt(values ...any) int {
	for _, v := range values {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}

func routeInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func frameError(i int, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &RecordingError{Frame: i, Index: i, At: i, Step: i, Err: err}
}

func (rec *Recording) verify(route []Visit, assets map[string]Asset, groups map[string]Set, pinned Set, consumers map[string][]string, budget int64) (*recordingOutcome, error) {
	if rec == nil {
		return nil, nil
	}
	frameCount := len(rec.Frames)
	rep := Replay{loaded: make(Set)}
	out := &recordingOutcome{replay: rep, history: make(history)}
	routePos := -1
	targets := make([]int, 0, frameCount)
	leave := make(map[string]int)
	lost := make(Set)
	for i, f := range rec.Frames {
		target, err := f.targetVisit(route, i, routePos)
		if err != nil {
			return nil, frameError(i, "%w", err)
		}
		previousRoutePos := routePos
		if i > 0 && target != routePos && target != routePos+1 {
			return nil, frameError(i, "route position cannot skip or move backwards")
		}
		advanced := i == 0 || target == previousRoutePos+1
		routePos = target
		targets = append(targets, target)

		crossedSector := ""
		if advanced && target > 0 && route[target].Sector != route[target-1].Sector {
			crossedSector = route[target-1].Sector
			leave[crossedSector] = i
			// A later revisit and fresh departure starts a new lookback life.
			delete(lost, crossedSector)
		}

		for opIndex, op := range f.Operations {
			if _, ok := assets[op.Asset]; !ok {
				return nil, frameError(i, "operation %d references unknown asset %q", opIndex, op.Asset)
			}
			if op.Action != "load" && op.Action != "evict" {
				return nil, frameError(i, "operation %d has unknown action %q", opIndex, op.Action)
			}
			if err := rep.apply(op, assets, consumers, budget); err != nil {
				return nil, frameError(i, "operation %d: %w", opIndex, err)
			}
			// Complete residency is continuous: once an eviction breaks a
			// departed sector's closure, a later reload does not restore its
			// lookback eligibility.
			for sector, d := range leave {
				if d <= i && !lost[sector] && !hasAll(rep.loaded, groups[sector]) {
					lost[sector] = true
				}
			}
		}
		if crossedSector != "" && !hasAll(rep.loaded, groups[crossedSector]) {
			lost[crossedSector] = true
		}

		required := make(Set)
		required.merge(groups[route[target].Sector])
		required.merge(pinned)
		if !hasAll(rep.loaded, required) {
			missing := missingIDs(required, rep.loaded)
			return nil, frameError(i, "frame end is missing required/pinned asset(s): %s", strings.Join(missing, ","))
		}
		out.endSets = append(out.endSets, rep.loaded.clone())
	}

	pause, pauseProvided, err := rec.pauseSet()
	if err != nil {
		errFrame := frameCount - 1
		if errFrame < 0 {
			errFrame = 0
		}
		return nil, frameError(errFrame, "%w", err)
	}
	if !pauseProvided {
		// A capture must carry the engine-confirmed pause set. An empty capture
		// has the implicit empty set.
		if frameCount > 0 {
			err := frameError(frameCount-1, "recording is missing the engine-confirmed pause resident set")
			return nil, err
		}
		pause = make(Set)
	}
	if err := knownSetAssets(pause, assets); err != nil {
		errFrame := frameCount - 1
		if errFrame < 0 {
			errFrame = 0
		}
		return nil, frameError(errFrame, "%w", err)
	}
	actual := rep.loaded
	if !setsEqual(pause, actual) {
		errFrame := frameCount - 1
		if errFrame < 0 {
			errFrame = 0
		}
		return nil, frameError(errFrame, "confirmed resident set does not match replayed operations (missing=%s unexpected=%s)",
			strings.Join(setDifference(actual, pause).sorted(), ","), strings.Join(setDifference(pause, actual).sorted(), ","))
	}

	// Recompute byte total from the confirmed set. This also protects against
	// any bookkeeping drift in the operation replay.
	var total int64
	for id := range pause {
		total += assets[id].Bytes
	}
	rep.loaded = pause.clone()
	rep.bytes = total

	// Use actual departure frame markers. Keep only sectors whose complete
	// closure never lost full residency after their latest departure.
	for sector, d := range leave {
		if !lost[sector] {
			out.history[sector] = d
		}
	}

	out.nextRoute = routePos + 1
	if frameCount == 0 {
		out.nextRoute = 0
	}
	out.nextLive = frameCount
	out.replay = rep
	out.pause = pause
	_ = targets
	return out, nil
}

func (rec *Recording) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	var frames []RecordedFrame
	frameDec := json.NewDecoder(bytes.NewReader(trimmed))
	frameDec.DisallowUnknownFields()
	if err := frameDec.Decode(&frames); err == nil {
		rec.Frames = frames
		return nil
	}
	type alias Recording
	var a alias
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	*rec = Recording(a)
	return nil
}

func (r *Recording) pauseSet() (Set, bool, error) {
	ids := firstSlice(r.Resident, r.Residency, r.ResidentSet, r.Residents, r.Paused,
		r.PauseSet, r.PausedSet, r.PauseResidency, r.PausedResident,
		r.PauseResident, r.PausedResidents, r.Confirmed, r.ConfirmedSet,
		r.ConfirmedResident, r.EngineResident, r.EngineResidency,
		r.FinalResident, r.EndResident)
	if ids == nil {
		// The pause set may be carried by the final frame.
		if len(r.Frames) > 0 {
			last := r.Frames[len(r.Frames)-1]
			ids = firstSlice(last.Resident, last.Residents, last.EndResident, last.EndResidency)
		}
	}
	if ids == nil {
		return nil, false, nil
	}
	out := make(Set, len(ids))
	seen := make(Set)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out[id] = true
	}
	return out, true, nil
}

func knownSetAssets(ids Set, assets map[string]Asset) error {
	var unknown []string
	for id := range ids {
		if _, ok := assets[id]; !ok {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("resident set references unknown asset(s): %s", strings.Join(unknown, ","))
	}
	return nil
}

func hasAll(loaded Set, required Set) bool {
	for id := range required {
		if !loaded[id] {
			return false
		}
	}
	return true
}

func missingIDs(required, loaded Set) []string {
	var out []string
	for id := range required {
		if !loaded[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func setsEqual(a, b Set) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}

func setDifference(a, b Set) Set {
	out := make(Set)
	for id := range a {
		if !b[id] {
			out[id] = true
		}
	}
	return out
}

func (s Set) clone() Set {
	out := make(Set, len(s))
	for id, v := range s {
		out[id] = v
	}
	return out
}

// UnmarshalJSON accepts the small set of common operation field aliases while
// still rejecting genuinely unknown fields at the JSON boundary.
func (o *Op) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var rawString string
	if err := json.Unmarshal(data, &rawString); err == nil {
		action, asset, ok := parseStringOp(rawString)
		if !ok {
			return fmt.Errorf("invalid operation %q", rawString)
		}
		o.Action, o.Asset = action, asset
		return nil
	}
	var rawPair []string
	if err := json.Unmarshal(data, &rawPair); err == nil && len(rawPair) == 2 {
		o.Action, o.Asset = rawPair[0], rawPair[1]
		if strings.EqualFold(o.Action, "unload") {
			o.Action = "evict"
		}
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{
		"action": true, "type": true, "kind": true, "op": true, "operation": true,
		"asset": true, "id": true, "assetid": true, "assetname": true, "target": true,
	}
	for key := range fields {
		if !allowed[strings.ToLower(key)] {
			return fmt.Errorf("json: unknown operation field %q", key)
		}
	}
	getString := func(keys ...string) string {
		for _, key := range keys {
			if raw, ok := fields[key]; ok {
				var s string
				if err := json.Unmarshal(raw, &s); err == nil {
					return s
				}
			}
			for actual, raw := range fields {
				if strings.EqualFold(actual, key) {
					var s string
					if err := json.Unmarshal(raw, &s); err == nil {
						return s
					}
				}
			}
		}
		return ""
	}
	o.Action = getString("Action", "Type", "Kind", "Op", "Operation")
	o.Asset = getString("Asset", "ID", "AssetID", "AssetName", "Target")
	switch strings.ToLower(o.Action) {
	case "unload", "evict", "eviction":
		o.Action = "evict"
	case "load", "loaded":
		o.Action = "load"
	}
	return nil
}

func parseStringOp(s string) (action, asset string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	for _, sep := range []string{":", " ", "\t"} {
		if i := strings.Index(s, sep); i >= 0 {
			left := strings.ToLower(strings.TrimSpace(s[:i]))
			right := strings.TrimSpace(s[i+len(sep):])
			if left == "+" || left == "load" {
				return "load", right, right != ""
			}
			if left == "-" || left == "unload" || left == "evict" {
				return "evict", right, right != ""
			}
		}
	}
	if strings.HasPrefix(s, "+") {
		return "load", strings.TrimPrefix(s, "+"), true
	}
	if strings.HasPrefix(s, "-") {
		return "evict", strings.TrimPrefix(s, "-"), true
	}
	return "", "", false
}
