package domain

import (
	"fmt"
	"sort"
)

// interval is a free half-open range [Start, End) inside the single heap.
type interval struct {
	Start, End int64
}

// heapState models one contiguous address heap with an ordered free list.
// Fixed offsets are carved out at construction and never returned to the pool.
type heapState struct {
	capacity int64
	free     []interval
}

func newHeap(capacity int64) *heapState {
	return &heapState{capacity: capacity, free: []interval{{0, capacity}}}
}

func (h *heapState) clone() *heapState {
	cp := &heapState{capacity: h.capacity, free: make([]interval, len(h.free))}
	copy(cp.free, h.free)
	return cp
}

// reserve permanently removes [at, at+size) from the pool (fixed-address assets).
func (h *heapState) reserve(at, size int64) error {
	for i, f := range h.free {
		if at >= f.Start && at+size <= f.End {
			nf := make([]interval, 0, len(h.free)+1)
			nf = append(nf, h.free[:i]...)
			if f.Start < at {
				nf = append(nf, interval{f.Start, at})
			}
			if at+size < f.End {
				nf = append(nf, interval{at + size, f.End})
			}
			nf = append(nf, h.free[i+1:]...)
			h.free = nf
			return nil
		}
	}
	return fmt.Errorf("fixed range [%d,%d) overlaps another reservation or leaves the heap", at, at+size)
}

// allocExact reserves [at, at+size); the caller already checked alignment.
func (h *heapState) allocExact(at, size int64) bool {
	for i, f := range h.free {
		if at >= f.Start && at+size <= f.End {
			nf := make([]interval, 0, len(h.free)+1)
			nf = append(nf, h.free[:i]...)
			if f.Start < at {
				nf = append(nf, interval{f.Start, at})
			}
			if at+size < f.End {
				nf = append(nf, interval{at + size, f.End})
			}
			nf = append(nf, h.free[i+1:]...)
			h.free = nf
			return true
		}
	}
	return false
}

// allocFirstFit returns the lowest aligned address whose range fits, or -1.
func (h *heapState) allocFirstFit(size, align int64) int64 {
	for _, f := range h.free {
		at := alignUp(f.Start, align)
		if at+size <= f.End {
			return at
		}
	}
	return -1
}

func (h *heapState) alloc(size, align int64) int64 {
	at := h.allocFirstFit(size, align)
	if at < 0 {
		return -1
	}
	if !h.allocExact(at, size) {
		return -1
	}
	return at
}

// release returns a range to the pool and merges with neighbouring holes.
func (h *heapState) release(start, end int64) {
	h.free = append(h.free, interval{start, end})
	sort.Slice(h.free, func(i, j int) bool { return h.free[i].Start < h.free[j].Start })
	merged := h.free[:0]
	for _, f := range h.free {
		if n := len(merged); n > 0 && merged[n-1].End >= f.Start {
			if f.End > merged[n-1].End {
				merged[n-1].End = f.End
			}
			continue
		}
		merged = append(merged, f)
	}
	h.free = merged
}

func alignUp(v, align int64) int64 {
	return (v + align - 1) / align * align
}

func isPowerOfTwo(v int64) bool { return v > 0 && v&(v-1) == 0 }
