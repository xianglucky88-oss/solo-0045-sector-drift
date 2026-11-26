package domain

import "fmt"

func compile(r Request) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget}
	if err := validate(r); err != nil {
		return plan, err
	}
	show, err := prepareShow(r)
	if err != nil {
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
	availability, err := availabilityMap(&r)
	if err != nil {
		return plan, err
	}
	gates, err := connect(r.Gates, sectors)
	if err != nil {
		return plan, err
	}
	if err := verifyRoute(r.Route, sectors, gates, availability); err != nil {
		return plan, err
	}
	order, err := dependencyOrder(assets)
	if err != nil {
		return plan, err
	}
	consumers := reverseEdges(assets)
	groups := sectorClosures(sectors, assets)
	pinned := closure(r.Policy.Pinned, assets)
	plan.Pinned = pinned.sorted()
	plan.Crossings = crossings(r.Route, gates, availability)

	// First compile exactly the historical per-frame policy.  These sets are
	// the mandatory/pinned/retain/lookahead allocation to which warmup may add.
	baseSets := make([]Set, 0, len(r.Route))
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
		frame := Frame{i, v, groups[v.Sector].sorted(), next.sorted(), replay.bytes, ops, decisions}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)
		baseSets = append(baseSets, next)
	}

	if !show.active {
		return plan, nil
	}

	opportunities := prepareOpportunities(show, r, gates, availability, groups)
	selected := selectOpportunities(opportunities, baseSets, assets, r.Policy.Budget)
	bookings := make([]Booking, 0)
	finalSets := make([]Set, len(r.Route))
	for i := range r.Route {
		finalSets[i] = make(Set)
		finalSets[i].merge(baseSets[i])
	}
	for i, ok := range selected {
		if !ok {
			continue
		}
		o := opportunities[i]
		bookings = append(bookings, bookingFromOpportunity(o))
		for f := o.start; f <= o.arrival; f++ {
			finalSets[f].merge(o.demand)
		}
	}
	assignScheduleOutput(&plan, bookings, show)

	// Replay the union of historical residency and fulfilled warmup holds.
	plan.Usage = Usage{}
	warmReplay := Replay{loaded: make(Set)}
	for i := range r.Route {
		next := finalSets[i]
		ops := diff(order, warmReplay.loaded, next)
		for _, op := range ops {
			if err := warmReplay.apply(op, assets, consumers, r.Policy.Budget); err != nil {
				return plan, err
			}
		}
		frame := plan.Frames[i]
		frame.Operations = ops
		frame.Resident = warmReplay.loaded.sorted()
		frame.Bytes = warmReplay.bytes
		plan.Frames[i] = frame
		plan.Usage.record(frame, assets)
	}
	return plan, nil
}

func assignScheduleOutput(plan *Plan, bookings []Booking, cfg showConfig) {
	if bookings == nil {
		bookings = []Booking{}
	}
	plan.Schedule = bookings
	plan.Show = bookings
	plan.ShowSchedule = bookings
	plan.Performance = bookings
	plan.Productions = bookings
	plan.Bookings = bookings
	plan.Fulfilled = bookings
	plan.FulfilledOpportunities = bookings
	plan.Delivered = bookings
	plan.Warmups = bookings
	plan.Warmup = bookings
	plan.WarmupSchedule = bookings
	plan.Setups = bookings
	plan.Shots = bookings
	plan.CompletedShots = bookings
	plan.Framing = bookings
	plan.FramingOpportunities = bookings
	plan.Opportunities = bookings
	plan.AcceptedOpportunities = bookings
	plan.Selected = bookings
	plan.Selection = bookings
	plan.Accepted = bookings
	plan.PhotoOps = bookings
	plan.Cues = bookings
	plan.Calls = bookings
	plan.CallSheet = bookings
	plan.Vistas = bookings
	plan.WarmupPlan = bookings
	plan.Preloads = bookings
	plan.PreloadSchedule = bookings
	plan.Holds = bookings
	plan.Residencies = bookings
	plan.GateShots = bookings
	plan.DoorShots = bookings
	plan.ShotBookings = bookings
	plan.Honored = bookings
	plan.Kept = bookings
	plan.Realized = bookings
	result := &ScheduleResult{
		LeadFrames: cfg.lead, WindowFrames: cfg.window, Count: len(bookings),
		Bookings: bookings, Fulfilled: bookings, Opportunities: bookings, Schedule: bookings, Warmups: bookings,
	}
	plan.ScheduleResult = result
	plan.WarmupResult = result
	plan.ShowResult = result
}
