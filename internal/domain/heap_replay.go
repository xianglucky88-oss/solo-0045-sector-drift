package domain

import (
	"fmt"
	"sort"
)

// replayHeap independently re-executes the emitted plan from an empty heap and
// verifies every engine-level invariant. It is run as part of compilation so a
// plan can only be published when it is fully replayable.
type heapReplay struct {
	capacity int64
	assets   map[string]Asset
	resident map[string]interval
}

func newHeapReplay(capacity int64, assets map[string]Asset) *heapReplay {
	return &heapReplay{capacity: capacity, assets: assets, resident: make(map[string]interval)}
}

func (r *heapReplay) apply(op Op) error {
	a, ok := r.assets[op.Asset]
	if !ok {
		return fmt.Errorf("unknown asset %q", op.Asset)
	}
	size := a.Bytes
	switch op.Action {
	case "load":
		if _, exists := r.resident[a.ID]; exists {
			return fmt.Errorf("double load %s", a.ID)
		}
		if op.To == nil {
			return fmt.Errorf("load %s missing target address", a.ID)
		}
		for _, d := range a.After {
			if _, ok := r.resident[d]; !ok {
				return fmt.Errorf("load %s before dependency %s", a.ID, d)
			}
		}
		seg := interval{*op.To, *op.To + size}
		if err := r.checkSegment(a, seg, true); err != nil {
			return fmt.Errorf("load %s: %w", a.ID, err)
		}
		r.resident[a.ID] = seg
	case "move":
		cur, exists := r.resident[a.ID]
		if !exists {
			return fmt.Errorf("move of absent asset %s", a.ID)
		}
		if op.From == nil || *op.From != cur.Start {
			return fmt.Errorf("move %s does not start from its current address", a.ID)
		}
		if op.To == nil {
			return fmt.Errorf("move %s missing target address", a.ID)
		}
		if a.FixedOffset != nil {
			return fmt.Errorf("fixed asset %s cannot be moved", a.ID)
		}
		seg := interval{*op.To, *op.To + size}
		// The source stays occupied for the whole copy, so it is excluded from
		// the free-space collision check rather than removed up front.
		if err := r.checkSegment(a, seg, false); err != nil {
			return fmt.Errorf("move %s: %w", a.ID, err)
		}
		r.resident[a.ID] = seg
	case "evict":
		cur, exists := r.resident[a.ID]
		if !exists {
			return fmt.Errorf("eviction of absent asset %s", a.ID)
		}
		if op.From == nil || *op.From != cur.Start {
			return fmt.Errorf("evict %s does not start from its current address", a.ID)
		}
		for id := range r.resident {
			if id == a.ID {
				continue
			}
			for _, d := range r.assets[id].After {
				if d == a.ID {
					return fmt.Errorf("eviction would orphan %s", id)
				}
			}
		}
		delete(r.resident, a.ID)
	default:
		return fmt.Errorf("unknown operation %q", op.Action)
	}
	return nil
}

// checkSegment validates bounds, alignment (fixed offsets included) and that
// the segment only overlaps free space; when includeSelf is false the asset's
// own current segment is allowed to overlap (the move copy window).
func (r *heapReplay) checkSegment(a Asset, seg interval, includeSelf bool) error {
	align := effectiveAlign(a)
	if seg.Start < 0 || seg.End > r.capacity || seg.Start%align != 0 {
		return fmt.Errorf("segment [%d,%d) violates heap bounds or alignment", seg.Start, seg.End)
	}
	if a.FixedOffset != nil && seg.Start != *a.FixedOffset {
		return fmt.Errorf("fixed asset placed at %d instead of %d", seg.Start, *a.FixedOffset)
	}
	for id, other := range r.resident {
		if !includeSelf && id == a.ID {
			continue
		}
		if seg.Start < other.End && other.Start < seg.End {
			return fmt.Errorf("segment [%d,%d) overlaps resident %s", seg.Start, seg.End, id)
		}
	}
	return nil
}

// verifyPlan replays all frames end to end and checks each frame-end layout.
func verifyHeapPlan(plan Plan, assets map[string]Asset) error {
	r := newHeapReplay(plan.Heap.Capacity, assets)
	for _, f := range plan.Frames {
		for _, op := range f.Operations {
			if err := r.apply(op); err != nil {
				return fmt.Errorf("frame %d: %w", f.Index, err)
			}
		}
		if len(r.resident) != len(f.Resident) {
			return fmt.Errorf("frame %d: replay residency %d does not match plan %d", f.Index, len(r.resident), len(f.Resident))
		}
		ids := make([]string, 0, len(r.resident))
		for id := range r.resident {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for j, id := range ids {
			if id != f.Resident[j] {
				return fmt.Errorf("frame %d: replay resident set diverges", f.Index)
			}
		}
		if len(f.Layout) != len(r.resident) {
			return fmt.Errorf("frame %d: layout does not cover every resident", f.Index)
		}
		for _, pl := range f.Layout {
			seg, ok := r.resident[pl.Asset]
			if !ok || seg.Start != pl.Offset || seg.End-seg.Start != pl.Size {
				return fmt.Errorf("frame %d: layout entry for %s disagrees with replay", f.Index, pl.Asset)
			}
		}
		var total int64
		for _, seg := range r.resident {
			total += seg.End - seg.Start
		}
		if total != f.Bytes || total > r.capacity {
			return fmt.Errorf("frame %d: byte accounting %d disagrees (layout %d, capacity %d)", f.Index, f.Bytes, total, r.capacity)
		}
		for _, req := range f.Required {
			if _, ok := r.resident[req]; !ok {
				return fmt.Errorf("frame %d: required asset %s not resident at frame end", f.Index, req)
			}
		}
	}
	return nil
}
