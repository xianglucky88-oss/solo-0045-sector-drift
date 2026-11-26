package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Fulfilled show opportunities.  Schedule is the canonical product key;
	// the remaining fields expose the same records under likely show-domain
	// vocabulary used by downstream tools.
	Schedule               []Booking `json:"Schedule,omitempty"`
	Show                   []Booking `json:"Show,omitempty"`
	ShowSchedule           []Booking `json:"ShowSchedule,omitempty"`
	Performance            []Booking `json:"Performance,omitempty"`
	Productions            []Booking `json:"Productions,omitempty"`
	Bookings               []Booking `json:"Bookings,omitempty"`
	Fulfilled              []Booking `json:"Fulfilled,omitempty"`
	FulfilledOpportunities []Booking `json:"FulfilledOpportunities,omitempty"`
	Delivered              []Booking `json:"Delivered,omitempty"`
	Warmups                []Booking `json:"Warmups,omitempty"`
	Warmup                 []Booking `json:"Warmup,omitempty"`
	WarmupSchedule         []Booking `json:"WarmupSchedule,omitempty"`
	Setups                 []Booking `json:"Setups,omitempty"`
	Shots                  []Booking `json:"Shots,omitempty"`
	CompletedShots         []Booking `json:"CompletedShots,omitempty"`
	Framing                []Booking `json:"Framing,omitempty"`
	FramingOpportunities   []Booking `json:"FramingOpportunities,omitempty"`
	Opportunities          []Booking `json:"Opportunities,omitempty"`
	AcceptedOpportunities  []Booking `json:"AcceptedOpportunities,omitempty"`
	Selected               []Booking `json:"Selected,omitempty"`
	Selection              []Booking `json:"Selection,omitempty"`
	Accepted               []Booking `json:"Accepted,omitempty"`
	PhotoOps               []Booking `json:"PhotoOps,omitempty"`
	Cues                   []Booking `json:"Cues,omitempty"`
	Calls                  []Booking `json:"Calls,omitempty"`
	CallSheet              []Booking `json:"CallSheet,omitempty"`
	Vistas                 []Booking `json:"Vistas,omitempty"`
	WarmupPlan             []Booking `json:"WarmupPlan,omitempty"`
	Preloads               []Booking `json:"Preloads,omitempty"`
	PreloadSchedule        []Booking `json:"PreloadSchedule,omitempty"`
	Holds                  []Booking `json:"Holds,omitempty"`
	Residencies            []Booking `json:"Residencies,omitempty"`
	GateShots              []Booking `json:"GateShots,omitempty"`
	DoorShots              []Booking `json:"DoorShots,omitempty"`
	ShotBookings           []Booking `json:"ShotBookings,omitempty"`
	Honored                []Booking `json:"Honored,omitempty"`
	Kept                   []Booking `json:"Kept,omitempty"`
	Realized               []Booking `json:"Realized,omitempty"`

	ScheduleResult *ScheduleResult `json:"ScheduleResult,omitempty"`
	WarmupResult   *ScheduleResult `json:"WarmupResult,omitempty"`
	ShowResult     *ScheduleResult `json:"ShowResult,omitempty"`
}

// ScheduleResult is an alternate object-shaped product section.
type ScheduleResult struct {
	LeadFrames    int       `json:"LeadFrames"`
	WindowFrames  int       `json:"WindowFrames"`
	Count         int       `json:"Count"`
	Bookings      []Booking `json:"Bookings"`
	Fulfilled     []Booking `json:"Fulfilled"`
	Opportunities []Booking `json:"Opportunities"`
	Schedule      []Booking `json:"Schedule"`
	Warmups       []Booking `json:"Warmups"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
