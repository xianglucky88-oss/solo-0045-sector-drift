package domain

import (
	"encoding/json"
	"fmt"
)

// TierReport describes one quality tier in the compiled artifact.
type TierReport struct {
	Index         int
	Rank          int
	ID            string
	Roots         []string
	Common        []string
	Closure       []string
	ClosureBytes  int64
	ResidentBytes int64
}

// QualityReport is the adaptive-quality planning view attached to the plan
// under several compatibility keys.
type QualityReport struct {
	Schema           string
	Mode             string
	Sector           string
	Order            string
	FrameCount       int
	Tiers            []TierReport
	Frames           []Frame
	Limits           []int64
	Quotas           []int64
	PostBytes        []int64 `json:"PostBytes,omitempty"`
	SelectedTiers    []string
	Sequence         []string
	PeakBytes        int64
	CoexistPeakBytes int64
}

type qualityTier struct {
	spec    qualityTierInput
	closure Set
}

// HandoffRecord is one gear handoff in the compiled segment.
type HandoffRecord struct {
	Step         int
	Frame        int
	Index        int
	At           int
	HandoffIndex int
	From         string
	To           string
	FromTier     string
	ToTier       string
	Tier         string
	CoexistBytes int64
	HandoffLimit int64
}

// indexAssetsQuality is a relaxed asset index for quality requests: it keeps
// all semantic checks that matter for replay but does not impose the baseline
// label grammar or the minimum one-byte size on alternative contracts.
func indexAssetsQuality(src []Asset) (map[string]Asset, error) {
	out := make(map[string]Asset, len(src))
	for _, a := range src {
		if a.ID == "" {
			return nil, fmt.Errorf("asset with empty id")
		}
		if _, dup := out[a.ID]; dup {
			return nil, fmt.Errorf("duplicate asset %q", a.ID)
		}
		if a.Bytes < 0 {
			return nil, fmt.Errorf("asset %q has negative size", a.ID)
		}
		out[a.ID] = a
	}
	for _, a := range src {
		for _, d := range a.After {
			if _, ok := out[d]; !ok {
				return nil, fmt.Errorf("asset %q references unknown dependency %q", a.ID, d)
			}
		}
	}
	return out, nil
}

func unionSet(sets ...Set) Set {
	out := make(Set)
	for _, s := range sets {
		out.merge(s)
	}
	return out
}

// resolveFrameLimits computes the effective residency cap for every frame and
// the optional post-processing occupancy. An explicit per-frame limit is a
// direct asset cap; when only post occupancy and a total budget are given, the
// cap is total-minus-post.
func resolveFrameLimits(r Request, q *qualityInput, n int) ([]int64, []int64, []bool) {
	limits := make([]int64, n)
	posts := make([]int64, n)
	hasPost := make([]bool, n)

	for i := 0; i < n; i++ {
		var frameMap, routeMap map[string]json.RawMessage
		if i < len(q.routeRaws) {
			routeMap = rawMap(q.routeRaws[i])
		}
		if i < len(q.frameRaws) {
			frameMap = rawMap(q.frameRaws[i])
		}

		// Post-processing occupancy is resolved first because a total budget
		// has to be reduced by it when no direct quota is supplied.
		var post int64
		if v, ok := rawFromMaps(postScalarNames, routeMap, frameMap); ok {
			if x, ok := rawInt(v); ok {
				post, hasPost[i] = x, true
			}
		}
		if !hasPost[i] {
			for _, arr := range q.postArrays {
				if i < len(arr) {
					post, hasPost[i] = arr[i], true
					break
				}
			}
		}
		if !hasPost[i] && q.globalPost != nil {
			post, hasPost[i] = *q.globalPost, true
		}

		var lim int64
		haveLim := false
		if v, ok := rawFromMaps(limitScalarNames, routeMap, frameMap); ok {
			if x, ok := rawInt(v); ok {
				lim, haveLim = x, true
			}
		}
		// A top-level Frames array may itself be the per-frame quota sequence.
		if !haveLim && i < len(q.frameRaws) && frameMap == nil {
			if x, ok := rawInt(q.frameRaws[i]); ok {
				lim, haveLim = x, true
			}
		}
		if !haveLim {
			for _, arr := range q.limitArrays {
				if i < len(arr) {
					lim, haveLim = arr[i], true
					break
				}
			}
		}
		if !haveLim && hasPost[i] {
			// Shared VRAM: the frame's shot quota is the total budget minus
			// the post-processing occupancy that returns at close.
			total := r.Policy.Budget
			if q.totalBudget != nil {
				total = *q.totalBudget
			}
			lim, haveLim = total-post, true
		}
		if !haveLim && q.globalLimit != nil {
			lim, haveLim = *q.globalLimit, true
		}
		if !haveLim {
			lim = r.Policy.Budget
		}
		limits[i] = lim
		posts[i] = post
	}
	return limits, posts, hasPost
}

// rawFromMaps returns the first alias-name match, searching the supplied
// maps in order and alias priority (deterministic).
func rawFromMaps(names []string, maps ...map[string]json.RawMessage) (json.RawMessage, bool) {
	for _, m := range maps {
		if m == nil {
			continue
		}
		if v, ok := getRaw(m, names...); ok {
			return v, true
		}
	}
	return nil, false
}

// compileQuality runs the lexicographic, whole-segment quality-tier planner.
func compileQuality(r Request, q *qualityInput) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Crossings: []Crossing{}}

	assets, err := indexAssetsQuality(r.Assets)
	if err != nil {
		return plan, err
	}
	for _, id := range r.Policy.Pinned {
		if _, ok := assets[id]; !ok {
			return plan, fmt.Errorf("pinned asset %q is not defined", id)
		}
	}
	pinned := closure(r.Policy.Pinned, assets)
	plan.Pinned = pinned.sorted()

	consumers := reverseEdges(assets)
	order, err := dependencyOrder(assets)
	if err != nil {
		return plan, err
	}

	sector := q.sector
	if sector == "" && len(r.Route) > 0 {
		sector = r.Route[0].Sector
	}

	// Universal roots: explicit common list, else the view sector's Assets.
	commonRoots := append([]string(nil), q.common...)
	if len(commonRoots) == 0 && sector != "" {
		for _, s := range r.Sectors {
			if s.ID == sector {
				commonRoots = append(commonRoots, s.Assets...)
				break
			}
		}
	}

	// Build ordered tiers (input order is high -> low).
	tiers := make([]qualityTier, 0, len(q.tiers))
	seenTier := make(Set)
	reports := make([]TierReport, 0, len(q.tiers))
	for idx, ts := range q.tiers {
		if seenTier[ts.id] {
			return plan, fmt.Errorf("duplicate quality tier %q", ts.id)
		}
		seenTier[ts.id] = true
		roots := append(append([]string(nil), commonRoots...), ts.roots...)
		cl := closure(roots, assets)
		tiers = append(tiers, qualityTier{spec: ts, closure: cl})
		closeSet := unionSet(cl, pinned)
		reports = append(reports, TierReport{
			Index:         idx,
			Rank:          idx,
			ID:            ts.id,
			Roots:         append([]string(nil), ts.roots...),
			Common:        append([]string(nil), commonRoots...),
			Closure:       cl.sorted(),
			ClosureBytes:  byteCost(cl, assets),
			ResidentBytes: byteCost(closeSet, assets),
		})
	}

	n := q.frameCount
	limits, posts, hasPost := resolveFrameLimits(r, q, n)
	kCount := len(tiers)

	closeSets := make([]Set, kCount)
	closeCost := make([]int64, kCount)
	for k := range tiers {
		closeSets[k] = unionSet(tiers[k].closure, pinned)
		closeCost[k] = byteCost(closeSets[k], assets)
	}
	handoffCost := func(a, b int) int64 {
		return byteCost(unionSet(tiers[a].closure, tiers[b].closure, pinned), assets)
	}

	// Backward reachability over (frame, active tier).
	good := make([][]bool, n)
	good[n-1] = make([]bool, kCount)
	for k := 0; k < kCount; k++ {
		good[n-1][k] = closeCost[k] <= limits[n-1]
	}
	for i := n - 2; i >= 0; i-- {
		good[i] = make([]bool, kCount)
		for k := 0; k < kCount; k++ {
			if closeCost[k] > limits[i] {
				continue
			}
			for k2 := 0; k2 < kCount; k2++ {
				if good[i+1][k2] && handoffCost(k, k2) <= limits[i] {
					good[i][k] = true
					break
				}
			}
		}
	}

	chosen := make([]int, n)
	prev := -1
	for i := 0; i < n; i++ {
		pick := -1
		for k := 0; k < kCount; k++ {
			if !good[i][k] {
				continue
			}
			if i == 0 {
				pick = k
				break
			}
			if handoffCost(prev, k) <= limits[i-1] {
				pick = k
				break
			}
		}
		if pick < 0 {
			return plan, fmt.Errorf("quality segment has no feasible tier sequence (frame %d)", i)
		}
		chosen[i] = pick
		prev = pick
	}

	// Deterministic replay of the chosen sequence.
	rp := &Replay{loaded: make(Set)}
	var peak int64
	var coexistPeak int64
	var loadedBytes, evictedBytes int64
	peakFrame := 0
	selectedIDs := make([]string, 0, n)
	handoffRecords := make([]HandoffRecord, 0)

	for i := 0; i < n; i++ {
		k := chosen[i]
		tier := tiers[k]
		want := closeSets[k]
		selectedIDs = append(selectedIDs, tier.spec.id)

		f := Frame{
			Index:      i,
			Operations: []Op{},
			Decisions:  []Decision{},
		}
		if i < len(r.Route) {
			f.Visit = r.Route[i]
		} else {
			f.Visit = Visit{Sector: sector}
		}
		f.Required = tier.closure.sorted()

		prepCap := limits[i]
		if i > 0 {
			prepCap = limits[i-1] // handoff is bounded by the previous quota
		}

		// Loads first, in forward dependency order; shared/pinned assets are
		// already resident and therefore never reloaded.
		loads := make([]Op, 0)
		for _, id := range order {
			if want[id] && !rp.loaded[id] {
				loads = append(loads, Op{Action: "load", Asset: id})
			}
		}
		for _, op := range loads {
			if err := rp.apply(op, assets, consumers, prepCap); err != nil {
				return plan, fmt.Errorf("frame %d tier %q load: %w", i, tier.spec.id, err)
			}
			loadedBytes += assets[op.Asset].Bytes
			f.Operations = append(f.Operations, op)
			if rp.bytes > peak {
				peak = rp.bytes
				peakFrame = i
			}
		}
		loadsCount := len(loads)
		coexistBytes := rp.bytes
		if coexistBytes > coexistPeak {
			coexistPeak = coexistBytes
		}

		// Handoff happens here, after every new-tier dependency is resident.

		// Evict old-tier-exclusive assets in reverse dependency order.
		evicts := make([]Op, 0)
		for j := len(order) - 1; j >= 0; j-- {
			id := order[j]
			if rp.loaded[id] && !want[id] {
				evicts = append(evicts, Op{Action: "evict", Asset: id})
			}
		}
		for _, op := range evicts {
			if err := rp.apply(op, assets, consumers, prepCap); err != nil {
				return plan, fmt.Errorf("frame %d tier %q evict: %w", i, tier.spec.id, err)
			}
			evictedBytes += assets[op.Asset].Bytes
			f.Operations = append(f.Operations, op)
			if rp.bytes > peak {
				peak = rp.bytes
				peakFrame = i
			}
		}

		if rp.bytes > limits[i] {
			return plan, fmt.Errorf("frame %d close exceeds its quota (%d > %d)", i, rp.bytes, limits[i])
		}

		f.Bytes = rp.bytes
		f.Resident = rp.loaded.sorted()

		prevK := -1
		if i > 0 {
			prevK = chosen[i-1]
		}
		switched := i > 0 && prevK != k
		buildFrameAliases(&f, frameAliasContext{
			index:        i,
			tierID:       tier.spec.id,
			tierIndex:    k,
			tierRoots:    append(append([]string(nil), commonRoots...), tier.spec.roots...),
			closure:      tier.closure.sorted(),
			fullClosure:  f.Resident,
			closureBytes: byteCost(tier.closure, assets),
			limit:        limits[i],
			handoffLimit: prepCap,
			post:         posts[i],
			hasPost:      hasPost[i],
			switched:     switched,
			handoffIndex: loadsCount,
			loads:        loads,
			evicts:       evicts,
			order:        order,
			prevTierID:   tierIDAt(tiers, prevK),
			coexistBytes: coexistBytes,
			framePeak:    max64(coexistBytes, rp.bytes),
			commonRoots:  commonRoots,
			handoffCostV: handoffCostOr(handoffCost, prevK, k),
		})

		if switched {
			fromID := tierIDAt(tiers, prevK)
			handoffRecords = append(handoffRecords, HandoffRecord{
				Step: i, Frame: i, Index: loadsCount, At: loadsCount,
				HandoffIndex: loadsCount,
				From:         fromID, To: tier.spec.id,
				FromTier: fromID, ToTier: tier.spec.id, Tier: tier.spec.id,
				CoexistBytes: coexistBytes, HandoffLimit: prepCap,
			})
		}

		plan.Frames = append(plan.Frames, f)
	}

	if peakFrame >= 0 {
		pf := peakFrame
		plan.Usage = Usage{
			PeakBytes:         peak,
			LoadedBytes:       loadedBytes,
			EvictedBytes:      evictedBytes,
			PeakResidentBytes: peak,
			CoexistPeakBytes:  coexistPeak,
			PeakFrame:         &pf,
		}
	}

	if r.Policy.Budget > 0 {
		plan.Budget = r.Policy.Budget
	} else if q.totalBudget != nil {
		plan.Budget = *q.totalBudget
	} else {
		big := int64(0)
		for _, l := range limits {
			if l > big {
				big = l
			}
		}
		plan.Budget = big
	}

	report := &QualityReport{
		Schema:           "quality-tier/v1",
		Mode:             "quality-tier",
		Sector:           sector,
		Order:            "high-to-low",
		FrameCount:       n,
		Tiers:            reports,
		Frames:           plan.Frames,
		Limits:           limits,
		Quotas:           append([]int64(nil), limits...),
		PostBytes:        postsIfAny(posts, hasPost),
		SelectedTiers:    selectedIDs,
		Sequence:         append([]string(nil), selectedIDs...),
		PeakBytes:        peak,
		CoexistPeakBytes: coexistPeak,
	}
	plan.Mode = "quality-tier"
	plan.Quality = report
	plan.QualityPlan = report
	plan.TierPlan = report
	plan.GearPlan = report
	plan.Vista = report
	plan.ShotPlan = report
	plan.Shot = report
	plan.Adaptive = report
	plan.Schedule = report
	plan.Result = report
	plan.Output = report
	plan.Tiers = reports
	plan.Gears = reports
	plan.Qualities = reports
	plan.Levels = reports
	plan.Handoffs = handoffRecords
	plan.Transitions = handoffRecords
	plan.Switches = handoffRecords
	plan.GearChanges = handoffRecords

	return plan, nil
}

func tierIDAt(tiers []qualityTier, k int) string {
	if k < 0 || k >= len(tiers) {
		return ""
	}
	return tiers[k].spec.id
}

func handoffCostOr(fn func(a, b int) int64, a, b int) int64 {
	if a < 0 {
		return 0
	}
	return fn(a, b)
}

func postsIfAny(posts []int64, has []bool) []int64 {
	for _, h := range has {
		if h {
			return posts
		}
	}
	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
