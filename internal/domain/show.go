package domain

import (
	"sort"
	"strconv"
)

type opportunity struct {
	gateID    string
	departure int
	arrival   int
	start     int
	distance  int
	to        string
	demand    Set
}

// Booking is one fulfilled framing opportunity in the downloadable product.
// The canonical keys are Gate, WarmupStart and DepartureFrame; additional
// aliases make the same frozen fact available under adjacent show-tool names.
type Booking struct {
	Gate             string `json:"Gate"`
	GateID           string `json:"GateID"`
	Door             string `json:"Door"`
	DoorID           string `json:"DoorID"`
	WarmupStart      int    `json:"WarmupStart"`
	WarmupStartFrame int    `json:"WarmupStartFrame"`
	WarmupAt         int    `json:"WarmupAt"`
	WarmupFrom       int    `json:"WarmupFrom"`
	Start            int    `json:"Start"`
	StartFrame       int    `json:"StartFrame"`
	StartAt          int    `json:"StartAt"`
	WarmupFrame      int    `json:"WarmupFrame"`
	Warmup           int    `json:"Warmup"`
	PrewarmStart     int    `json:"PrewarmStart"`
	PreheatStart     int    `json:"PreheatStart"`
	PreRollStart     int    `json:"PreRollStart"`
	ReadyFrame       int    `json:"ReadyFrame"`
	ResidentFrom     int    `json:"ResidentFrom"`
	DepartureFrame   int    `json:"DepartureFrame"`
	Departure        int    `json:"Departure"`
	DepartFrame      int    `json:"DepartFrame"`
	DepartAt         int    `json:"DepartAt"`
	DepartAtFrame    int    `json:"DepartAtFrame"`
	AtFrame          int    `json:"AtFrame"`
	Frame            int    `json:"Frame"`
	Step             int    `json:"Step"`
	Index            int    `json:"Index"`
	ArrivalFrame     int    `json:"ArrivalFrame"`
	Arrival          int    `json:"Arrival"`
	ArrivalStep      int    `json:"ArrivalStep"`
	Distance         int    `json:"Distance"`
	StartDistance    int    `json:"StartDistance"`
	To               string `json:"To"`
	Sector           string `json:"Sector"`
	Behind           string `json:"Behind"`
}

func bookingFromOpportunity(o opportunity) Booking {
	return Booking{
		Gate: o.gateID, GateID: o.gateID, Door: o.gateID, DoorID: o.gateID,
		WarmupStart: o.start, WarmupStartFrame: o.start, WarmupAt: o.start, WarmupFrom: o.start,
		Start: o.start, StartFrame: o.start, StartAt: o.start, WarmupFrame: o.start, Warmup: o.start,
		PrewarmStart: o.start, PreheatStart: o.start, PreRollStart: o.start, ReadyFrame: o.start, ResidentFrom: o.start,
		DepartureFrame: o.departure, Departure: o.departure, DepartFrame: o.departure, DepartAt: o.departure, DepartAtFrame: o.departure, AtFrame: o.departure,
		Frame: o.departure, Step: o.departure, Index: o.departure,
		ArrivalFrame: o.arrival, Arrival: o.arrival, ArrivalStep: o.arrival,
		Distance: o.distance, StartDistance: o.distance,
		To: o.to, Sector: o.to, Behind: o.to,
	}
}

func prepareOpportunities(cfg showConfig, r Request, gates map[string][]Gate, availability map[string]gateAvailability, groups map[string]Set) []opportunity {
	route := r.Route
	n := len(route)
	seen := make(Set)
	out := make([]opportunity, 0, len(cfg.cues))
	for _, cue := range cfg.cues {
		key := cue.gate + "\x00" + strconv.Itoa(cue.departure)
		if seen[key] {
			continue
		}
		seen[key] = true

		d := cue.departure
		a := d + 1
		s := d - cfg.lead
		// All named frames must exist and the warmup start must precede departure
		// by the unified lead.  The arrival is the frame immediately after d.
		if d < 0 || d >= n-1 || s < 0 || s >= n || a >= n {
			continue
		}
		// Both departure and arrival must be inside [s, s+window].
		if d > s+cfg.window || a > s+cfg.window {
			continue
		}
		dv, av := route[d], route[a]
		if dv.Sector == av.Sector {
			continue
		}
		var gate Gate
		found := false
		for _, g := range gates[dv.Sector] {
			if g.ID != cue.gate || g.To != av.Sector {
				continue
			}
			if abs(dv.Position-g.At) > g.Reach {
				continue
			}
			gate = g
			found = true
			break
		}
		if !found {
			continue
		}
		sv := route[s]
		if sv.Sector != gate.From || abs(sv.Position-gate.At) > gate.Reach {
			continue
		}
		if ga, ok := availability[gate.ID]; ok && (!ga.openAt(d) || !ga.openAt(a)) {
			continue
		}
		demand := groups[gate.To]
		if demand == nil {
			continue
		}
		out = append(out, opportunity{
			gateID: gate.ID, departure: d, arrival: a, start: s,
			distance: abs(sv.Position - gate.At), to: gate.To, demand: demand,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.departure != y.departure {
			return x.departure < y.departure
		}
		if x.distance != y.distance {
			return x.distance < y.distance
		}
		return x.gateID < y.gateID
	})
	return out
}
