package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func ptr(v int64) *int64 { return &v }

func TestHeapFreeList(t *testing.T) {
	h := newHeap(100)
	if got := h.alloc(30, 1); got != 0 {
		t.Fatalf("first alloc = %d, want 0", got)
	}
	if got := h.alloc(20, 8); got != 32 {
		t.Fatalf("aligned alloc = %d, want 32", got)
	}
	h.release(0, 30)
	// Reused low hole wins over the high free interval.
	if got := h.alloc(10, 1); got != 0 {
		t.Fatalf("reuse alloc = %d, want 0", got)
	}
	// Releasing adjacent holes merges them; the aligned block [32,52) stays
	// allocated so the remaining free space is one continuous [10,100) hole.
	h.release(10, 30)
	h.release(32, 52)
	if len(h.free) != 1 || h.free[0] != (interval{10, 100}) {
		t.Fatalf("free list after merge = %+v", h.free)
	}
	if got := h.alloc(200, 1); got != -1 {
		t.Fatalf("oversize alloc = %d, want -1", got)
	}
}

func TestHeapFixedReservationsNeverReturn(t *testing.T) {
	h := newHeap(64)
	if err := h.reserve(8, 8); err != nil {
		t.Fatal(err)
	}
	// First fit reuses the hole in front of the fixed [8,16) reservation.
	if got := h.alloc(8, 8); got != 0 {
		t.Fatalf("alloc before fixed slot = %d, want 0", got)
	}
	if err := h.reserve(8, 8); err == nil {
		t.Fatal("overlapping fixed reservation was accepted")
	}
}

// baseRequest builds a two-sector world with a shared dependency, suitable for
// heap-focused tests. Sizes are intentionally chosen so fragmentation matters.
func baseRequest() Request {
	return Request{
		Sectors: []Sector{
			{ID: "one", Assets: []string{"a", "b"}},
			{ID: "two", Assets: []string{"c", "d"}},
		},
		Assets: []Asset{
			{ID: "z", Bytes: 10},
			{ID: "a", Bytes: 30, After: []string{"z"}},
			{ID: "b", Bytes: 40, After: []string{"z"}},
			{ID: "c", Bytes: 20, After: []string{"z"}},
			{ID: "d", Bytes: 60, After: []string{"z"}},
		},
		Gates: []Gate{{ID: "g1", From: "one", To: "two", At: 50, Reach: 100}},
		Route: []Visit{
			{Sector: "one", Position: 10},
			{Sector: "two", Position: 10},
		},
		Policy: Policy{Budget: 1000, Lookahead: 0, Retain: 0, Heap: &HeapPolicy{Capacity: 120}},
	}
}

func TestHeapPlanReplayAndMove(t *testing.T) {
	r := baseRequest()
	// z is a fixed pin in the middle of the heap so holes on either side cannot
	// merge; d only fits after b is relocated toward the low hole (a real move).
	r.Policy.Pinned = []string{"z"}
	r.Assets[0].FixedOffset = ptr(60) // z at [60,70), immovable
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if plan.Heap == nil || plan.Heap.Capacity != 120 {
		t.Fatalf("heap report missing: %+v", plan.Heap)
	}
	if err := verifyHeapPlan(plan, indexAssetsOrFail(t, r.Assets)); err != nil {
		t.Fatalf("plan failed independent replay: %v", err)
	}
	var moved []Op
	for _, f := range plan.Frames {
		for _, op := range f.Operations {
			if op.Action == "move" {
				moved = append(moved, op)
				if op.From == nil || op.To == nil || *op.From == *op.To {
					t.Fatalf("move needs distinct source and target addresses: %+v", op)
				}
			}
			if op.Action == "load" && op.To == nil {
				t.Fatalf("load without target address: %+v", op)
			}
			if op.Action == "evict" && op.From == nil {
				t.Fatalf("evict without source address: %+v", op)
			}
		}
		if len(f.Layout) != len(f.Resident) {
			t.Fatalf("frame %d layout/resident mismatch", f.Index)
		}
	}
	if len(moved) == 0 {
		t.Fatal("expected at least one relocation to make d fit, got none")
	}
	for _, op := range moved {
		if op.Asset == "z" {
			t.Fatal("fixed asset z was moved")
		}
	}
	// Every frame-end layout must stay inside the heap and never overlap.
	for _, f := range plan.Frames {
		var prevEnd int64
		for _, pl := range f.Layout {
			if pl.Offset < prevEnd {
				t.Fatalf("frame %d overlapping layout: %+v", f.Index, f.Layout)
			}
			if pl.Offset+pl.Size > 120 {
				t.Fatalf("frame %d placement leaves the heap", f.Index)
			}
			prevEnd = pl.Offset + pl.Size
		}
	}
}

func TestHeapFirstFrameOffsetBindsOnce(t *testing.T) {
	r := baseRequest()
	r.Assets[1].FirstOffset = ptr(80) // a: [80,110) on frame 0 only
	r.Policy.Heap.Capacity = 120
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	f0 := plan.Frames[0]
	var loadA *Op
	for i := range f0.Operations {
		if f0.Operations[i].Asset == "a" {
			loadA = &f0.Operations[i]
		}
	}
	if loadA == nil || loadA.To == nil || *loadA.To != 80 {
		t.Fatalf("a not loaded at its first-frame offset: %+v", f0.Operations)
	}
	if err := verifyHeapPlan(plan, indexAssetsOrFail(t, r.Assets)); err != nil {
		t.Fatalf("replay: %v", err)
	}
	// After a is evicted on frame 1, its first-frame slot becomes ordinary
	// free space and must not carry any special reservation in the layout.
	for _, pl := range plan.Frames[1].Layout {
		if pl.Asset == "a" {
			t.Fatal("a should not be resident on frame 1")
		}
	}
}

func TestHeapFirstOffsetRequiresFrameZeroMandatory(t *testing.T) {
	r := baseRequest()
	// c belongs to sector two, so it is not mandatory on frame 0.
	r.Assets[3].FirstOffset = ptr(0)
	if _, err := compile(r); err == nil ||
		!strings.Contains(err.Error(), "first-frame offset") {
		t.Fatalf("expected first-frame eligibility error, got %v", err)
	}
}

func TestHeapOverlappingFirstOffsetsRejected(t *testing.T) {
	r := baseRequest()
	r.Assets[1].FirstOffset = ptr(0)  // a [0,30)
	r.Assets[2].FirstOffset = ptr(10) // b [10,50) overlaps
	if _, err := compile(r); err == nil ||
		!strings.Contains(err.Error(), "first-frame offset overlaps") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
}

func TestHeapAlignmentEnforced(t *testing.T) {
	r := baseRequest()
	r.Assets[1].Align = 16
	r.Policy.Heap.Capacity = 120
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	assets := indexAssetsOrFail(t, r.Assets)
	for _, f := range plan.Frames {
		for _, pl := range f.Layout {
			if pl.Offset%effectiveAlign(assets[pl.Asset]) != 0 {
				t.Fatalf("frame %d asset %s misaligned at %d", f.Index, pl.Asset, pl.Offset)
			}
		}
	}
	if err := verifyHeapPlan(plan, assets); err != nil {
		t.Fatalf("replay: %v", err)
	}
}

func TestHeapImpossibleHandoffNamesFirstFrame(t *testing.T) {
	r := baseRequest()
	// Frame 0 fits (z+a+b = 80). On frame 1 the mandatory closure z+c+d needs
	// 90 bytes, while fixed pins permanently consume 40 bytes of a 120-byte
	// heap, so no sequence of in-heap relocations can make it fit.
	r.Policy.Pinned = []string{"z", "wall"}
	r.Assets[0].FixedOffset = ptr(0) // z [0,10)
	r.Assets = append(r.Assets, Asset{ID: "wall", Bytes: 35, FixedOffset: ptr(85)})
	r.Policy.Heap.Capacity = 120
	_, err := compile(r)
	if err == nil || !strings.Contains(err.Error(), "frame 1") {
		t.Fatalf("expected frame 1 handoff failure, got %v", err)
	}
}

func TestHeapMandatoryOversizeRejectedAtFrameZero(t *testing.T) {
	r := baseRequest()
	r.Policy.Heap.Capacity = 20
	_, err := compile(r)
	if err == nil || !strings.Contains(err.Error(), "frame 0") {
		t.Fatalf("expected frame 0 rejection, got %v", err)
	}
}

func TestHeapCandidateRollbackKeepsLaterCandidates(t *testing.T) {
	r := baseRequest()
	// Add a third sector reachable from one. It is tried as an optional
	// candidate and must roll back cleanly when its closure cannot fit.
	r.Sectors = append(r.Sectors, Sector{ID: "three", Assets: []string{"bulk"}})
	r.Assets = append(r.Assets, Asset{ID: "bulk", Bytes: 300})
	r.Gates = append(r.Gates, Gate{ID: "g2", From: "one", To: "three", At: 10, Reach: 100})
	r.Policy.Lookahead = 100
	r.Policy.Heap.Capacity = 120
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	found := false
	for _, d := range plan.Frames[0].Decisions {
		if d.Candidate.Sector == "three" && !d.Admitted {
			found = true
		}
	}
	if !found {
		t.Fatal("oversized candidate was not deferred")
	}
	if err := verifyHeapPlan(plan, indexAssetsOrFail(t, r.Assets)); err != nil {
		t.Fatalf("replay after rollback: %v", err)
	}
}

func TestHeapDeterminism(t *testing.T) {
	r := baseRequest()
	r.Policy.Pinned = []string{"z"}
	r.Assets[0].FixedOffset = ptr(60)
	first, err := Build(r)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	// Shuffle every input slice; the emitted plan must be byte-identical.
	r2 := r
	r2.Assets = append([]Asset(nil), r.Assets...)
	for i, j := 0, len(r2.Assets)-1; i < j; i, j = i+1, j-1 {
		r2.Assets[i], r2.Assets[j] = r2.Assets[j], r2.Assets[i]
	}
	r2.Sectors = append([]Sector(nil), r.Sectors...)
	r2.Sectors[0], r2.Sectors[1] = r2.Sectors[1], r2.Sectors[0]
	second, err := Build(r2)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("heap plans differ for the same shuffled input")
	}
}

func TestHeapZeroCapacityFallsBackToBudget(t *testing.T) {
	r := baseRequest()
	r.Policy.Heap = &HeapPolicy{Capacity: 0}
	r.Policy.Budget = 120
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if plan.Heap == nil || plan.Heap.Capacity != 120 {
		t.Fatalf("expected heap capacity 120 from budget, got %+v", plan.Heap)
	}
}

func TestNonHeapRequestKeepsLegacyShape(t *testing.T) {
	r := baseRequest()
	r.Policy.Heap = nil
	r.Policy.Budget = 200
	plan, err := compile(r)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if plan.Heap != nil {
		t.Fatal("legacy request must not carry a heap report")
	}
	for _, f := range plan.Frames {
		if f.Layout != nil {
			t.Fatal("legacy request must not carry heap layouts")
		}
		for _, op := range f.Operations {
			if op.From != nil || op.To != nil {
				t.Fatalf("legacy op must not carry addresses: %+v", op)
			}
		}
	}
}

func TestHeapPlanJSONShape(t *testing.T) {
	r := baseRequest()
	raw, err := Build(r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	heap, ok := doc["Heap"].(map[string]any)
	if !ok || heap["Capacity"].(float64) != 120 {
		t.Fatalf("missing Heap block in JSON: %s", raw)
	}
	frames := doc["Frames"].([]any)
	f0 := frames[0].(map[string]any)
	layout, ok := f0["Layout"].([]any)
	if !ok || len(layout) == 0 {
		t.Fatal("frame 0 missing Layout")
	}
	first := layout[0].(map[string]any)
	for _, key := range []string{"Asset", "Offset", "Size"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("layout entry missing %s", key)
		}
	}
}

func indexAssetsOrFail(t *testing.T, src []Asset) map[string]Asset {
	t.Helper()
	out, err := indexAssets(src)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
