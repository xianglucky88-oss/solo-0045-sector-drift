package domain

import "fmt"

func compile(r Request) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget}
	if err := validate(r); err != nil {
		return plan, err
	}
	assets, err := indexAssets(r.Assets)
	if err != nil {
		return plan, err
	}
	if err := knownAssets(r.Policy.Pinned, assets); err != nil {
		return plan, err
	}
	sectors, err := indexSectors(r.Sectors, assets)
	if err != nil {
		return plan, err
	}
	gates, err := connect(r.Gates, sectors)
	if err != nil {
		return plan, err
	}
	if err := verifyRoute(r.Route, sectors, gates); err != nil {
		return plan, err
	}
	order, err := dependencyOrder(assets)
	if err != nil {
		return plan, err
	}
	consumers := reverseEdges(assets)
	groups := sectorClosures(sectors, assets)
	pinned := closure(r.Policy.Pinned, assets)
	cross := crossings(r.Route, gates)
	if r.Policy.Heap != nil {
		return compileHeap(r, assets, order, groups, pinned, cross, gates)
	}
	return compileBudget(r, assets, order, consumers, groups, pinned, cross, gates)
}

// compileBudget keeps the original byte-budget pipeline untouched for requests
// that do not opt into heap mode.
func compileBudget(r Request, assets map[string]Asset, order []string, consumers map[string][]string, groups map[string]Set, pinned Set, cross []Crossing, gates map[string][]Gate) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget, Pinned: pinned.sorted(), Crossings: cross}
	state := make(history)
	replay := Replay{loaded: make(Set)}
	for i, v := range r.Route {
		state.addStep(r.Route, i)
		options := prioritize(state.age(i, r.Policy.Retain), nearby(v, gates[v.Sector], r.Policy.Lookahead), v.Sector)
		next, decisions, err := admit(groups[v.Sector], pinned, options, groups, assets, r.Policy.Budget)
		if err != nil {
			return plan, fmt.Errorf("visit %d sector %s: %w", i, v.Sector, err)
		}
		ops := diff(order, replay.loaded, next)
		for _, op := range ops {
			if err := replay.apply(op, assets, consumers, r.Policy.Budget); err != nil {
				return plan, err
			}
		}
		frame := Frame{i, v, groups[v.Sector].sorted(), next.sorted(), replay.bytes, ops, decisions, nil}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)
	}
	return plan, nil
}

// compileHeap delivers an executable single-heap plan: placements, copies and
// evictions with concrete addresses, independently replayed before publishing.
func compileHeap(r Request, assets map[string]Asset, order []string, groups map[string]Set, pinned Set, cross []Crossing, gates map[string][]Gate) (Plan, error) {
	capacity := heapCapacity(r.Policy)
	mandatoryFrame0 := make(Set)
	mandatoryFrame0.merge(groups[r.Route[0].Sector])
	mandatoryFrame0.merge(pinned)
	heap, firstOffsets, err := validateHeap(r, capacity, assets, mandatoryFrame0)
	if err != nil {
		return Plan{}, err
	}
	planner := newHeapPlanner(r, assets, order, pinned, heap, capacity, cross, firstOffsets)
	state := make(history)
	for i, v := range r.Route {
		state.addStep(r.Route, i)
		options := prioritize(state.age(i, r.Policy.Retain), nearby(v, gates[v.Sector], r.Policy.Lookahead), v.Sector)
		if _, err := planner.frame(i, v, groups[v.Sector], options, groups); err != nil {
			return Plan{}, fmt.Errorf("visit %d sector %s: %w", i, v.Sector, err)
		}
	}
	plan := planner.plan
	if err := verifyHeapPlan(plan, assets); err != nil {
		return Plan{}, err
	}
	return plan, nil
}
