package domain

import (
	"fmt"
	"sort"
)

// validateHeap checks the static constraints of heap mode and carves fixed
// reservations into a fresh heap. First-frame offsets bind exactly one load:
// they are returned as frame-zero reservations and claimed during frame 0.
func validateHeap(r Request, capacity int64, assets map[string]Asset, mandatoryFrame0 Set) (*heapState, map[string]int64, error) {
	if capacity < 1 || capacity > 1<<40 {
		return nil, nil, fmt.Errorf("invalid heap capacity")
	}
	h := newHeap(capacity)
	firstOffsets := make(map[string]int64)
	reserved := make([]interval, 0)
	for _, id := range sortedAssetIDs(assets) {
		a := assets[id]
		if a.Align < 0 || (a.Align != 0 && !isPowerOfTwo(a.Align)) {
			return nil, nil, fmt.Errorf("asset %s declares non power-of-two alignment", id)
		}
		align := effectiveAlign(a)
		size := a.Bytes
		if a.FixedOffset != nil {
			at := *a.FixedOffset
			seg := interval{at, at + size}
			if at < 0 || at%align != 0 || at+size > capacity {
				return nil, nil, fmt.Errorf("asset %s has an illegal fixed offset", id)
			}
			if overlapsAny(seg, reserved) {
				return nil, nil, fmt.Errorf("asset %s fixed offset overlaps another reservation", id)
			}
			if err := h.reserve(at, size); err != nil {
				return nil, nil, fmt.Errorf("asset %s: %w", id, err)
			}
			reserved = append(reserved, seg)
		}
		if a.FirstOffset != nil {
			at := *a.FirstOffset
			seg := interval{at, at + size}
			if !mandatoryFrame0[id] {
				return nil, nil, fmt.Errorf("asset %s declares a first-frame offset but is not mandatory on frame 0", id)
			}
			if at < 0 || at%align != 0 || at+size > capacity {
				return nil, nil, fmt.Errorf("asset %s has an illegal first-frame offset", id)
			}
			if a.FixedOffset != nil && *a.FixedOffset != at {
				return nil, nil, fmt.Errorf("asset %s declares conflicting fixed and first-frame offsets", id)
			}
			if a.FixedOffset == nil {
				if overlapsAny(seg, reserved) {
					return nil, nil, fmt.Errorf("asset %s first-frame offset overlaps another reservation", id)
				}
				reserved = append(reserved, seg)
			}
			firstOffsets[id] = at
		}
	}
	return h, firstOffsets, nil
}

func overlapsAny(seg interval, others []interval) bool {
	for _, o := range others {
		if seg.Start < o.End && o.Start < seg.End {
			return true
		}
	}
	return false
}

// heapCapacity resolves the heap size: an explicit heap capacity wins, zero
// falls back to the streaming budget so existing policy inputs stay meaningful.
func heapCapacity(p Policy) int64 {
	if p.Heap != nil && p.Heap.Capacity > 0 {
		return p.Heap.Capacity
	}
	return p.Budget
}

func effectiveAlign(a Asset) int64 {
	if a.Align > 0 {
		return a.Align
	}
	return 1
}

func sortedAssetIDs(assets map[string]Asset) []string {
	ids := make([]string, 0, len(assets))
	for id := range assets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
