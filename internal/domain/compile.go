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
	limits := r.streamLimits()
	if limits.present {
		streamPlan, err := compileStream(r, assets, order, consumers, groups, pinned, limits)
		if err != nil {
			return plan, err
		}
		streamPlan.Crossings = crossings(r.Route, gates)
		if err := replayStreamPlan(streamPlan, assets, consumers, r.Policy.Budget, limits.effective()); err != nil {
			return plan, err
		}
		return streamPlan, nil
	}
	plan.Pinned = pinned.sorted()
	plan.Crossings = crossings(r.Route, gates)
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
		frame := Frame{
			Index:      i,
			Visit:      v,
			Required:   groups[v.Sector].sorted(),
			Resident:   next.sorted(),
			Bytes:      replay.bytes,
			Operations: ops,
			Decisions:  decisions,
		}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)
	}
	return plan, nil
}
