package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Performance/scheduling inputs.  The broad aliases keep the wire contract
	// tolerant of the vocabulary used by show-production tooling while the
	// canonical internal representation remains Show.
	Schedule             any `json:"Schedule,omitempty"`
	Show                 any `json:"Show,omitempty"`
	ShowSchedule         any `json:"ShowSchedule,omitempty"`
	Performance          any `json:"Performance,omitempty"`
	Production           any `json:"Production,omitempty"`
	Staging              any `json:"Staging,omitempty"`
	Cinematic            any `json:"Cinematic,omitempty"`
	Sequence             any `json:"Sequence,omitempty"`
	Framing              any `json:"Framing,omitempty"`
	Cues                 any `json:"Cues,omitempty"`
	Opportunities        any `json:"Opportunities,omitempty"`
	FramingOpportunities any `json:"FramingOpportunities,omitempty"`
	Shots                any `json:"Shots,omitempty"`
	PhotoOps             any `json:"PhotoOps,omitempty"`
	Vistas               any `json:"Vistas,omitempty"`
	Setups               any `json:"Setups,omitempty"`
	Bookings             any `json:"Bookings,omitempty"`
	Takes                any `json:"Takes,omitempty"`
	DoorShots            any `json:"DoorShots,omitempty"`
	CameraCues           any `json:"CameraCues,omitempty"`
	Calls                any `json:"Calls,omitempty"`
	CallSheet            any `json:"CallSheet,omitempty"`
	ShowPlan             any `json:"ShowPlan,omitempty"`
	SchedulePlan         any `json:"SchedulePlan,omitempty"`
	PerformancePlan      any `json:"PerformancePlan,omitempty"`
	FramingPlan          any `json:"FramingPlan,omitempty"`
	CameraPlan           any `json:"CameraPlan,omitempty"`
	ShotList             any `json:"ShotList,omitempty"`
	ShotSchedule         any `json:"ShotSchedule,omitempty"`
	WarmupPlan           any `json:"WarmupPlan,omitempty"`
	WarmupScheduleInput  any `json:"WarmupSchedule,omitempty"`
	PreloadSchedule      any `json:"PreloadSchedule,omitempty"`
	PreheatSchedule      any `json:"PreheatSchedule,omitempty"`
	CueSheet             any `json:"CueSheet,omitempty"`
	GateSchedule         any `json:"GateSchedule,omitempty"`
	DoorSchedule         any `json:"DoorSchedule,omitempty"`

	GateSchedules any `json:"GateSchedules,omitempty"`
	GateWindows   any `json:"GateWindows,omitempty"`
	OpenWindows   any `json:"OpenWindows,omitempty"`
	DoorSchedules any `json:"DoorSchedules,omitempty"`

	featureExtra map[string]any `json:"-"`
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

	// Frame availability.  Values are intentionally permissive at the boundary;
	// normalizeShow converts all supported spellings/shapes.
	Open             any `json:"Open,omitempty"`
	OpenAt           any `json:"OpenAt,omitempty"`
	OpenOn           any `json:"OpenOn,omitempty"`
	OpenFrames       any `json:"OpenFrames,omitempty"`
	Frames           any `json:"Frames,omitempty"`
	OpenWindow       any `json:"OpenWindow,omitempty"`
	OpenWindows      any `json:"OpenWindows,omitempty"`
	Window           any `json:"Window,omitempty"`
	Windows          any `json:"Windows,omitempty"`
	OpenPeriod       any `json:"OpenPeriod,omitempty"`
	OpenPeriods      any `json:"OpenPeriods,omitempty"`
	Periods          any `json:"Periods,omitempty"`
	OpenInterval     any `json:"OpenInterval,omitempty"`
	OpenIntervals    any `json:"OpenIntervals,omitempty"`
	Intervals        any `json:"Intervals,omitempty"`
	OpenRange        any `json:"OpenRange,omitempty"`
	OpenRanges       any `json:"OpenRanges,omitempty"`
	Ranges           any `json:"Ranges,omitempty"`
	GateSchedule     any `json:"GateSchedule,omitempty"`
	Schedule         any `json:"Schedule,omitempty"`
	OpenSchedule     any `json:"OpenSchedule,omitempty"`
	Availability     any `json:"Availability,omitempty"`
	AvailableFrames  any `json:"AvailableFrames,omitempty"`
	ClosedFrames     any `json:"ClosedFrames,omitempty"`
	OpenFrameWindows any `json:"OpenFrameWindows,omitempty"`
	FrameWindows     any `json:"FrameWindows,omitempty"`
	OpeningWindows   any `json:"OpeningWindows,omitempty"`
	OpeningPeriods   any `json:"OpeningPeriods,omitempty"`
	OpeningIntervals any `json:"OpeningIntervals,omitempty"`
}

type Visit struct {
	Sector   string
	Position int
}
type Policy struct {
	Budget            int64
	Lookahead, Retain int
	Pinned            []string

	// Frame scheduling policy aliases.  Existing Lookahead remains the spatial
	// gate-candidate radius; these values are measured in route frames.
	WarmupFrames     any `json:"WarmupFrames,omitempty"`
	LeadFrames       any `json:"LeadFrames,omitempty"`
	PreRollFrames    any `json:"PreRollFrames,omitempty"`
	PrerollFrames    any `json:"PrerollFrames,omitempty"`
	PrewarmFrames    any `json:"PrewarmFrames,omitempty"`
	PreheatFrames    any `json:"PreheatFrames,omitempty"`
	AdvanceFrames    any `json:"AdvanceFrames,omitempty"`
	WarmupLeadFrames any `json:"WarmupLeadFrames,omitempty"`
	Warmup           any `json:"Warmup,omitempty"`
	WarmUp           any `json:"WarmUp,omitempty"`
	Lead             any `json:"Lead,omitempty"`
	PreRoll          any `json:"PreRoll,omitempty"`
	Preroll          any `json:"Preroll,omitempty"`
	Prewarm          any `json:"Prewarm,omitempty"`
	Preheat          any `json:"Preheat,omitempty"`
	Advance          any `json:"Advance,omitempty"`

	LookaheadFrames any `json:"LookaheadFrames,omitempty"`
	LookAheadFrames any `json:"LookAheadFrames,omitempty"`
	FrameLookahead  any `json:"FrameLookahead,omitempty"`
	LookaheadWindow any `json:"LookaheadWindow,omitempty"`
	WindowFrames    any `json:"WindowFrames,omitempty"`
	FrameWindow     any `json:"FrameWindow,omitempty"`
	WindowLength    any `json:"WindowLength,omitempty"`
	WindowSize      any `json:"WindowSize,omitempty"`
	Window          any `json:"Window,omitempty"`
	HorizonFrames   any `json:"HorizonFrames,omitempty"`
	Horizon         any `json:"Horizon,omitempty"`

	Schedule     any `json:"Schedule,omitempty"`
	Show         any `json:"Show,omitempty"`
	ShowSchedule any `json:"ShowSchedule,omitempty"`
	Framing      any `json:"Framing,omitempty"`
	Cues         any `json:"Cues,omitempty"`
	Shots        any `json:"Shots,omitempty"`

	GateWindows      any `json:"GateWindows,omitempty"`
	GateSchedules    any `json:"GateSchedules,omitempty"`
	OpenWindows      any `json:"OpenWindows,omitempty"`
	GateAvailability any `json:"GateAvailability,omitempty"`
}

// Schedule is the canonical show-scheduling document.  Every field is an any
// at the boundary so Go callers may use either convenient literals or the
// strongly typed Cue/FrameWindow helpers below.
type Schedule struct {
	Enabled any `json:"Enabled,omitempty"`
	Active  any `json:"Active,omitempty"`
	On      any `json:"On,omitempty"`

	WarmupFrames     any `json:"WarmupFrames,omitempty"`
	LeadFrames       any `json:"LeadFrames,omitempty"`
	PreRollFrames    any `json:"PreRollFrames,omitempty"`
	PrerollFrames    any `json:"PrerollFrames,omitempty"`
	PrewarmFrames    any `json:"PrewarmFrames,omitempty"`
	PreheatFrames    any `json:"PreheatFrames,omitempty"`
	AdvanceFrames    any `json:"AdvanceFrames,omitempty"`
	WarmupLeadFrames any `json:"WarmupLeadFrames,omitempty"`
	Warmup           any `json:"Warmup,omitempty"`
	WarmUp           any `json:"WarmUp,omitempty"`
	Lead             any `json:"Lead,omitempty"`
	PreRoll          any `json:"PreRoll,omitempty"`
	Preroll          any `json:"Preroll,omitempty"`
	Prewarm          any `json:"Prewarm,omitempty"`
	Preheat          any `json:"Preheat,omitempty"`
	Advance          any `json:"Advance,omitempty"`

	Lookahead       any `json:"Lookahead,omitempty"`
	LookaheadFrames any `json:"LookaheadFrames,omitempty"`
	LookAheadFrames any `json:"LookAheadFrames,omitempty"`
	FrameLookahead  any `json:"FrameLookahead,omitempty"`
	LookaheadWindow any `json:"LookaheadWindow,omitempty"`
	WindowFrames    any `json:"WindowFrames,omitempty"`
	FrameWindow     any `json:"FrameWindow,omitempty"`
	WindowLength    any `json:"WindowLength,omitempty"`
	WindowSize      any `json:"WindowSize,omitempty"`
	Window          any `json:"Window,omitempty"`
	HorizonFrames   any `json:"HorizonFrames,omitempty"`
	Horizon         any `json:"Horizon,omitempty"`

	Opportunities        any `json:"Opportunities,omitempty"`
	Cues                 any `json:"Cues,omitempty"`
	Shots                any `json:"Shots,omitempty"`
	PhotoOps             any `json:"PhotoOps,omitempty"`
	FramingOpportunities any `json:"FramingOpportunities,omitempty"`
	Vistas               any `json:"Vistas,omitempty"`
	Setups               any `json:"Setups,omitempty"`
	Bookings             any `json:"Bookings,omitempty"`
	Takes                any `json:"Takes,omitempty"`
	DoorShots            any `json:"DoorShots,omitempty"`
	CameraCues           any `json:"CameraCues,omitempty"`
	Items                any `json:"Items,omitempty"`

	Gates         any `json:"Gates,omitempty"`
	GateSchedules any `json:"GateSchedules,omitempty"`
	GateWindows   any `json:"GateWindows,omitempty"`
	OpenWindows   any `json:"OpenWindows,omitempty"`
}

// Cue is one framing opportunity.  Frame aliases all denote the departure
// frame; the following route frame is the arrival.
type Cue struct {
	Gate   any `json:"Gate,omitempty"`
	GateID any `json:"GateID,omitempty"`
	ID     any `json:"ID,omitempty"`
	Door   any `json:"Door,omitempty"`
	DoorID any `json:"DoorID,omitempty"`

	DepartureFrame any `json:"DepartureFrame,omitempty"`
	DepartFrame    any `json:"DepartFrame,omitempty"`
	Departure      any `json:"Departure,omitempty"`
	DepartAtFrame  any `json:"DepartAtFrame,omitempty"`
	DepartAt       any `json:"DepartAt,omitempty"`
	FromFrame      any `json:"FromFrame,omitempty"`
	StartFrame     any `json:"StartFrame,omitempty"`
	AtFrame        any `json:"AtFrame,omitempty"`
	Frame          any `json:"Frame,omitempty"`
	Step           any `json:"Step,omitempty"`
	Index          any `json:"Index,omitempty"`
	At             any `json:"At,omitempty"`
}

func (c *Cue) UnmarshalJSON(data []byte) error {
	data = normalizeTopLevelKeys(data)
	type alias Cue
	var a alias
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	*c = Cue(a)
	return nil
}

// Opportunity is an intentionally named alias for callers/tests using the
// requirement's vocabulary.
type Opportunity = Cue

// FrameWindow describes an inclusive frame interval by default.  Until/Before
// spellings are interpreted as an exclusive end.
type FrameWindow struct {
	Start        any `json:"Start,omitempty"`
	StartFrame   any `json:"StartFrame,omitempty"`
	Begin        any `json:"Begin,omitempty"`
	First        any `json:"First,omitempty"`
	From         any `json:"From,omitempty"`
	FromFrame    any `json:"FromFrame,omitempty"`
	Low          any `json:"Low,omitempty"`
	Left         any `json:"Left,omitempty"`
	Departure    any `json:"Departure,omitempty"`
	End          any `json:"End,omitempty"`
	EndFrame     any `json:"EndFrame,omitempty"`
	Finish       any `json:"Finish,omitempty"`
	Last         any `json:"Last,omitempty"`
	To           any `json:"To,omitempty"`
	ToFrame      any `json:"ToFrame,omitempty"`
	High         any `json:"High,omitempty"`
	Right        any `json:"Right,omitempty"`
	Stop         any `json:"Stop,omitempty"`
	Until        any `json:"Until,omitempty"`
	Before       any `json:"Before,omitempty"`
	ExclusiveEnd any `json:"ExclusiveEnd,omitempty"`
}

func (p *Policy) UnmarshalJSON(data []byte) error {
	data = normalizeTopLevelKeys(data)
	type alias Policy
	var a alias
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	*p = Policy(a)
	return nil
}

func (s *Schedule) UnmarshalJSON(data []byte) error {
	data = normalizeTopLevelKeys(data)
	var cues []Cue
	if err := json.Unmarshal(data, &cues); err == nil {
		*s = Schedule{Cues: cues}
		return nil
	}
	type alias Schedule
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = Schedule(a)
	return nil
}

func normalizeTopLevelKeys(data []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return data
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[normalizeKey(k)] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return data
	}
	return b
}

// UnmarshalJSON preserves Request's strict unknown-field contract while
// allowing schedule sections to use their permissive boundary types.
func (r *Request) UnmarshalJSON(data []byte) error {
	data = normalizeTopLevelKeys(data)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	known := knownRequestKeys()
	extra := make(map[string]any)
	for key, val := range raw {
		if known[key] || !featureLikeKey(key) {
			continue
		}
		var v any
		if err := json.Unmarshal(val, &v); err != nil {
			return err
		}
		extra[key] = v
		delete(raw, key)
	}
	filtered, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	type alias Request
	var a alias
	dec := json.NewDecoder(bytes.NewReader(filtered))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	*r = Request(a)
	r.featureExtra = extra
	if err := r.strictCheckFeatureDocuments(); err != nil {
		return err
	}
	return nil
}

func knownRequestKeys() map[string]bool {
	t := reflect.TypeOf(Request{})
	out := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Name
		tag := f.Tag.Get("json")
		if tag != "" && tag != "-" {
			if comma := strings.IndexByte(tag, ','); comma >= 0 {
				tag = tag[:comma]
			}
			if tag != "" {
				name = tag
			}
		}
		out[normalizeKey(name)] = true
	}
	return out
}

func featureLikeKey(key string) bool {
	nk := normalizeKey(key)
	for _, part := range []string{"schedule", "show", "cue", "call", "shot", "opportunit", "framing", "warm", "lead", "ahead", "advance", "window", "horizon", "span", "prewarm", "preheat", "preroll", "vista", "setup", "booking", "take", "performance", "production", "cinematic", "staging", "camera", "lookahead", "gate", "door", "preload"} {
		if strings.Contains(nk, part) {
			return true
		}
	}
	return false
}

func (g *Gate) UnmarshalJSON(data []byte) error {
	data = normalizeTopLevelKeys(data)
	type alias Gate
	var a alias
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return err
	}
	*g = Gate(a)
	return nil
}

func strictDecodeDocument(raw any, dst any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func (r *Request) strictCheckFeatureDocuments() error {
	containers := []any{
		r.Schedule, r.Show, r.ShowSchedule, r.Performance, r.Production, r.Staging, r.Cinematic, r.Sequence, r.Framing,
		r.ShowPlan, r.SchedulePlan, r.PerformancePlan, r.FramingPlan, r.CameraPlan, r.ShotSchedule, r.WarmupPlan,
		r.WarmupScheduleInput, r.PreloadSchedule, r.PreheatSchedule, r.CueSheet, r.CallSheet, r.GateSchedule, r.DoorSchedule, r.ShotList,
		r.Policy.Schedule, r.Policy.Show, r.Policy.ShowSchedule, r.Policy.Framing,
	}
	for _, c := range containers {
		if c == nil {
			continue
		}
		var s Schedule
		if err := strictDecodeDocument(c, &s); err != nil {
			return fmt.Errorf("schedule: %w", err)
		}
	}
	lists := []any{
		r.Cues, r.Opportunities, r.FramingOpportunities, r.Shots, r.PhotoOps, r.Vistas, r.Setups, r.Bookings,
		r.Takes, r.DoorShots, r.CameraCues, r.Calls, r.CallSheet,
		r.Policy.Cues, r.Policy.Shots,
	}
	for _, list := range lists {
		if list == nil {
			continue
		}
		var cues []Cue
		if err := strictDecodeDocument(list, &cues); err != nil {
			return fmt.Errorf("opportunities: %w", err)
		}
	}
	return nil
}
