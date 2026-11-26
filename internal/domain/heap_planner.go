package domain

import (
	"fmt"
	"sort"
)

// heapPlanner turns the existing sector/asset selection into an executable,
// single-heap residency plan. Every frame starts from the layout confirmed at
// the end of the previous frame; the heap itself is the source of truth for
// free space and placements track where each resident asset lives.
type heapPlanner struct {
	assets       map[string]Asset
	order        []string // topological order, dependencies first
	pinned       Set
	capacity     int64
	heap         *heapState
	placed       map[string]int64
	firstOffsets map[string]int64 // claimed exactly once during frame 0
	plan         Plan
}

type plannerSnapshot struct {
	heap         *heapState
	placed       map[string]int64
	firstOffsets map[string]int64
}

func newHeapPlanner(r Request, assets map[string]Asset, order []string, pinned Set, heap *heapState, capacity int64, crossings []Crossing, firstOffsets map[string]int64) *heapPlanner {
	// Frame-zero reservations hold first-frame offsets until their single load.
	// They are released to the ordinary pool when claimed or if never claimed.
	for id, at := range firstOffsets {
		if assets[id].FixedOffset == nil && !heap.allocExact(at, assets[id].Bytes) {
			panic("first-frame reservation already validated")
		}
	}
	return &heapPlanner{
		assets:       assets,
		order:        order,
		pinned:       pinned,
		capacity:     capacity,
		heap:         heap,
		placed:       make(map[string]int64),
		firstOffsets: firstOffsets,
		plan: Plan{
			Schema:    "sector-stream/v1",
			Budget:    r.Policy.Budget,
			Heap:      &HeapReport{Capacity: capacity},
			Pinned:    pinned.sorted(),
			Crossings: crossings,
		},
	}
}

func (p *heapPlanner) snapshot() plannerSnapshot {
	placed := make(map[string]int64, len(p.placed))
	for k, v := range p.placed {
		placed[k] = v
	}
	first := make(map[string]int64, len(p.firstOffsets))
	for k, v := range p.firstOffsets {
		first[k] = v
	}
	return plannerSnapshot{p.heap.clone(), placed, first}
}

func (p *heapPlanner) restore(s plannerSnapshot) {
	p.heap = s.heap
	p.placed = s.placed
	p.firstOffsets = s.firstOffsets
}

// loadOne loads an absent asset at a legal aligned free range. On first-fit
// failure it compacts every movable resident once and retries, so the only
// failures left are genuinely impossible handoffs.
func (p *heapPlanner) loadOne(id string, firstFrame bool, ops []Op) (int64, []Op, bool) {
	a := p.assets[id]
	align := effectiveAlign(a)
	if a.FixedOffset != nil {
		// The fixed range was carved out of the pool at construction, so the
		// load only records the placement; the range is never released either.
		at := *a.FixedOffset
		p.placed[id] = at
		ops = append(ops, Op{Action: "load", Asset: id, To: &at})
		return at, ops, true
	}
	if firstFrame && a.FirstOffset != nil {
		// The reservation was carved out at construction; claiming it records
		// the placement and retires the one-shot constraint.
		at := *a.FirstOffset
		delete(p.firstOffsets, id)
		p.placed[id] = at
		ops = append(ops, Op{Action: "load", Asset: id, To: &at})
		return at, ops, true
	}
	if at := p.heap.alloc(a.Bytes, align); at >= 0 {
		p.placed[id] = at
		ops = append(ops, Op{Action: "load", Asset: id, To: &at})
		return at, ops, true
	}
	ops = p.compact(ops)
	if at := p.heap.alloc(a.Bytes, align); at >= 0 {
		p.placed[id] = at
		ops = append(ops, Op{Action: "load", Asset: id, To: &at})
		return at, ops, true
	}
	// Fixed pins can split the heap so that packing low is not enough: relocate
	// movable residents (in either direction) until the pending range fits.
	if newOps, ok := p.makeRoom(a.Bytes, align, ops); ok {
		ops = newOps
		at := p.heap.alloc(a.Bytes, align)
		if at < 0 {
			return 0, ops, false
		}
		p.placed[id] = at
		ops = append(ops, Op{Action: "load", Asset: id, To: &at})
		return at, ops, true
	}
	return 0, ops, false
}

// movableOrder lists relocatable assets by current address, ties broken by ID.
func (p *heapPlanner) movableOrder() []string {
	movable := make([]string, 0, len(p.placed))
	for id := range p.placed {
		if p.assets[id].FixedOffset == nil {
			movable = append(movable, id)
		}
	}
	sort.Slice(movable, func(i, j int) bool {
		pi, pj := p.placed[movable[i]], p.placed[movable[j]]
		if pi != pj {
			return pi < pj
		}
		return movable[i] < movable[j]
	})
	return movable
}

// makeRoom performs goal-directed relocations until size/align fits. For every
// movable resident it simulates each aligned free destination on a cloned free
// list and only commits the first one that makes the pending range fit. The
// destination is always a currently free, aligned range disjoint from the
// source, so the source stays valid for the whole copy. All search orders are
// address-sorted, which keeps the plan deterministic.
func (p *heapPlanner) makeRoom(size, align int64, ops []Op) ([]Op, bool) {
	for iteration := 0; iteration < len(p.placed)+1; iteration++ {
		if p.heap.allocFirstFit(size, align) >= 0 {
			return ops, true
		}
		progressed := false
		for _, id := range p.movableOrder() {
			a := p.assets[id]
			from := p.placed[id]
			origin := interval{from, from + a.Bytes}
			for _, to := range p.relocationDestinations(a, origin) {
				trial := p.heap.clone()
				if !trial.allocExact(to, a.Bytes) {
					continue
				}
				trial.release(origin.Start, origin.End)
				if trial.allocFirstFit(size, align) < 0 {
					continue
				}
				p.heap = trial
				p.placed[id] = to
				ops = append(ops, Op{Action: "move", Asset: id, From: &from, To: &to})
				progressed = true
				break
			}
			if progressed {
				break
			}
		}
		if !progressed {
			return ops, false
		}
	}
	return ops, p.heap.allocFirstFit(size, align) >= 0
}

// relocationDestinations lists every aligned free range that fits a and is
// disjoint from its current segment, in ascending address order.
func (p *heapPlanner) relocationDestinations(a Asset, origin interval) []int64 {
	align := effectiveAlign(a)
	var out []int64
	for _, f := range p.heap.free {
		at := alignUp(f.Start, align)
		if at+a.Bytes > f.End {
			continue
		}
		seg := interval{at, at + a.Bytes}
		if seg.Start < origin.End && origin.Start < seg.End {
			continue
		}
		out = append(out, at)
	}
	return out
}

// compact packs every movable resident toward the lowest fitting aligned
// address. Each move copies into a fully free aligned hole first, then frees
// the origin; fixed-address assets and their reservations never move.
func (p *heapPlanner) compact(ops []Op) []Op {
	for _, id := range p.movableOrder() {
		a := p.assets[id]
		from := p.placed[id]
		align := effectiveAlign(a)
		to := p.heap.allocFirstFit(a.Bytes, align)
		if to < 0 || to >= from {
			continue
		}
		if !p.heap.allocExact(to, a.Bytes) {
			continue
		}
		p.heap.release(from, from+a.Bytes)
		p.placed[id] = to
		ops = append(ops, Op{Action: "move", Asset: id, From: &from, To: &to})
	}
	return ops
}

func (p *heapPlanner) evict(id string, ops []Op) []Op {
	a := p.assets[id]
	at := p.placed[id]
	if a.FixedOffset == nil {
		p.heap.release(at, at+a.Bytes)
	}
	delete(p.placed, id)
	ops = append(ops, Op{Action: "evict", Asset: id, From: &at})
	return ops
}

// loadSet loads every absent asset of the set in topological order.
func (p *heapPlanner) loadSet(set Set, firstFrame bool, ops []Op) ([]Op, bool) {
	var ok bool
	for _, id := range p.order {
		if !set[id] {
			continue
		}
		if _, present := p.placed[id]; present {
			continue
		}
		if _, ops, ok = p.loadOne(id, firstFrame, ops); !ok {
			return ops, false
		}
	}
	return ops, true
}

// evictOutside evicts placed assets that are not in keep, in reverse
// topological order so no dependency is ever orphaned.
func (p *heapPlanner) evictOutside(keep Set, ops []Op) []Op {
	for i := len(p.order) - 1; i >= 0; i-- {
		id := p.order[i]
		if _, present := p.placed[id]; present && !keep[id] {
			ops = p.evict(id, ops)
		}
	}
	return ops
}

// releaseUnclaimedFirstOffsets returns any one-shot frame-zero reservations
// that were not claimed to the ordinary free pool.
func (p *heapPlanner) releaseUnclaimedFirstOffsets() {
	for id, at := range p.firstOffsets {
		a := p.assets[id]
		if a.FixedOffset == nil {
			p.heap.release(at, at+a.Bytes)
		}
		delete(p.firstOffsets, id)
	}
}

// frame runs one visit and returns its frame, or an error naming the first
// frame whose mandatory residency cannot be handed over.
func (p *heapPlanner) frame(i int, v Visit, active Set, choices []Candidate, groups map[string]Set) (Frame, error) {
	mandatory := make(Set)
	mandatory.merge(active)
	mandatory.merge(p.pinned)
	firstFrame := i == 0
	ops := make([]Op, 0)

	// Phase A: assets that cannot stay regardless of optional admissions leave
	// first, in reverse topological order, so their space is truly reusable.
	ops = p.evictOutside(mandatory, ops)

	// Phase B: mandatory residency must be executable from this frame's start.
	var ok bool
	if ops, ok = p.loadSet(mandatory, firstFrame, ops); !ok {
		// Last resort inside the rules: give up every optional resident, pack
		// once more and retry. No off-heap space and no temporary unloading of
		// mandatory assets is ever used.
		ops = p.evictOutside(mandatory, ops)
		ops = p.compact(ops)
		if ops, ok = p.loadSet(mandatory, firstFrame, ops); !ok {
			return Frame{}, fmt.Errorf("frame %d: mandatory residency cannot be handed over inside the heap", i)
		}
	}
	// Frame-zero reservations that never got claimed become ordinary free space.
	if firstFrame {
		p.releaseUnclaimedFirstOffsets()
	}

	// Phase C: optional candidates keep the original retain/lookahead order.
	// Each candidate is tried on a cloned state; a failure rolls back and the
	// later candidates still get their chance.
	resident := make(Set)
	for id := range p.placed {
		resident[id] = true
	}
	decisions := make([]Decision, 0, len(choices))
	for _, c := range choices {
		group := groups[c.Sector]
		cost := incremental(group, resident, p.assets)
		snap := p.snapshot()
		opsBefore := len(ops)
		ops, ok = p.loadSet(group, firstFrame, ops)
		if ok {
			resident.merge(group)
			decisions = append(decisions, Decision{Candidate: c, Admitted: true, Increment: cost})
			continue
		}
		p.restore(snap)
		ops = ops[:opsBefore]
		decisions = append(decisions, Decision{Candidate: c, Admitted: false, Increment: cost})
	}

	// Phase D: anything still resident but not selected leaves before frame end.
	ops = p.evictOutside(resident, ops)

	frame := Frame{
		Index:      i,
		Visit:      v,
		Required:   mandatory.sorted(),
		Resident:   resident.sorted(),
		Bytes:      p.residentBytes(),
		Operations: ops,
		Decisions:  decisions,
		Layout:     p.layout(),
	}
	p.plan.Usage.record(frame, p.assets)
	p.plan.Frames = append(p.plan.Frames, frame)
	return frame, nil
}

func (p *heapPlanner) residentBytes() int64 {
	var total int64
	for id := range p.placed {
		total += p.assets[id].Bytes
	}
	return total
}

func (p *heapPlanner) layout() []Placement {
	out := make([]Placement, 0, len(p.placed))
	for id, at := range p.placed {
		out = append(out, Placement{Asset: id, Offset: at, Size: p.assets[id].Bytes})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Offset != out[j].Offset {
			return out[i].Offset < out[j].Offset
		}
		return out[i].Asset < out[j].Asset
	})
	return out
}
