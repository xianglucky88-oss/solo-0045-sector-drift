package domain

import (
	"encoding/json"
	"sort"
)

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	// Replay support. These sections are only embedded when the request
	// declares leases (or an inline replay); ordinary requests keep the
	// historical artifact shape byte-for-byte.
	Leases        leaseList     `json:"Leases,omitempty"`
	Assets        []Asset       `json:"Assets,omitempty"`
	Sectors       []Sector      `json:"Sectors,omitempty"`
	Gates         []Gate        `json:"Gates,omitempty"`
	Policy        *Policy       `json:"Policy,omitempty"`
	Replay        *ReplayReport `json:"Replay,omitempty"`
	LeaseReplay   *ReplayReport `json:"LeaseReplay,omitempty"`
	NominalReplay *ReplayReport `json:"NominalReplay,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	leases := collectLeases(r)
	enrichPlan(&plan, r, leases)
	hasEvents := r.Replay != nil || len(r.Events) > 0 || len(r.Timeline) > 0 || len(r.Trace) > 0
	if len(leases) > 0 || hasEvents {
		rr := ReplayRequest{}
		if r.Replay != nil {
			rr = *r.Replay
		}
		rr.Events = append(rr.Events, r.Events...)
		rr.Events = append(rr.Events, r.Timeline...)
		rr.Events = append(rr.Events, r.Trace...)
		if !hasEvents && len(leases) > 0 {
			// Nominal timeline: every declared lease is acquired at its
			// declared acquirable frame. Runtime traces override this. Order
			// by frame then credential so shuffled input digests identically.
			index, _ := indexAssets(r.Assets)
			nominal := canonicalLeases(leases, index)
			sort.SliceStable(nominal, func(i, j int) bool {
				if nominal[i].Frame != nominal[j].Frame {
					return nominal[i].Frame < nominal[j].Frame
				}
				return nominal[i].Credential < nominal[j].Credential
			})
			for _, l := range nominal {
				f := l.Frame
				rr.Events = append(rr.Events, LeaseEvent{
					Frame: f, HasFrame: true, Credential: l.Credential, Kind: EventAcquire,
				})
			}
		}
		rep, rerr := RunReplay(&plan, rr.Leases, rr)
		if rerr != nil {
			return nil, rerr
		}
		plan.Replay = rep
		plan.LeaseReplay = rep
		plan.NominalReplay = rep
	}
	return json.MarshalIndent(plan, "", "  ")
}
