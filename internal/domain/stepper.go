package domain

import "sort"

// enrichPlan embeds the graph data required for artifact-driven replay. Output
// slices are canonicalized so reordering input documents does not change the
// artifact digest.
func enrichPlan(plan *Plan, r Request, leases []Lease) {
	if len(leases) == 0 {
		return
	}

	assets := append([]Asset(nil), r.Assets...)
	sort.Slice(assets, func(i, j int) bool { return assets[i].ID < assets[j].ID })
	plan.Assets = assets

	index, _ := indexAssets(r.Assets)
	ll := canonicalLeases(leases, index)
	sort.SliceStable(ll, func(i, j int) bool { return leaseCred(ll[i]) < leaseCred(ll[j]) })
	plan.Leases = ll

	sectors := append([]Sector(nil), r.Sectors...)
	sort.Slice(sectors, func(i, j int) bool { return sectors[i].ID < sectors[j].ID })
	plan.Sectors = sectors

	gates := append([]Gate(nil), r.Gates...)
	sort.Slice(gates, func(i, j int) bool {
		if gates[i].ID != gates[j].ID {
			return gates[i].ID < gates[j].ID
		}
		if gates[i].From != gates[j].From {
			return gates[i].From < gates[j].From
		}
		return gates[i].To < gates[j].To
	})
	plan.Gates = gates

	policy := r.Policy
	plan.Policy = &policy
}

func leaseCred(l Lease) string {
	if l.Credential != "" {
		return l.Credential
	}
	if l.Token != "" {
		return l.Token
	}
	return l.ID
}

// Stepper drives a frame-by-frame stateful replay session.
type Stepper struct {
	eng    *leaseEngine
	Frames []ReplayFrame
	Next   int
	Digest string
	extra  []Lease
}

func NewStepper(plan *Plan, extra []Lease, digest string) (*Stepper, error) {
	eng, err := BuildEngine(plan, extra)
	if err != nil {
		return nil, err
	}
	return &Stepper{eng: eng, Digest: digest, extra: extra}, nil
}

// AdvanceTo processes empty frames up to frame-1 if needed, then processes
// events at the next frame. Requests for an already passed frame simply advance
// once more so a live caller can stream sequentially.
func (s *Stepper) AdvanceTo(frame int, events []LeaseEvent) (*ReplayFrame, error) {
	if frame < 0 {
		frame = s.Next
	}
	for s.Next < frame {
		if _, err := s.advanceOne(nil); err != nil {
			return nil, err
		}
	}
	return s.advanceOne(events)
}

func (s *Stepper) advanceOne(events []LeaseEvent) (*ReplayFrame, error) {
	rf, _, err := s.eng.Step(s.Next, events)
	if err != nil {
		return nil, err
	}
	s.Frames = append(s.Frames, rf)
	s.Next++
	return &rf, nil
}

// Through runs any remaining frames up to and including lastFrame without
// events, returning the full report.
func (s *Stepper) Through(lastFrame int) (*ReplayReport, error) {
	for s.Next <= lastFrame {
		if _, err := s.advanceOne(nil); err != nil {
			return nil, err
		}
	}
	return s.Report(), nil
}

func (s *Stepper) Report() *ReplayReport {
	results := append([]EventResult(nil), s.eng.allResults...)
	for i := range results {
		results[i].Index = i
	}
	summary := s.eng.summary
	summary.Frames = len(s.Frames)
	summary.PeakBytes = s.eng.peak
	summary.ConflictFrames = dedupeInts(summary.ConflictFrames)
	return &ReplayReport{
		Schema: "lease-replay/v1", Mode: "session", Kind: "lease-replay",
		Digest: s.Digest, OK: true, Budget: s.eng.budget,
		FrameCount: len(s.Frames), Frames: append([]ReplayFrame(nil), s.Frames...),
		Results: results, EventResults: results, Events: results, Items: results,
		Summary: summary, Usage: summary, Leases: s.extra,
	}
}
