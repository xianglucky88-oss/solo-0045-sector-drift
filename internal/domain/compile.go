package domain

import "fmt"

func compile(r Request) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget, Frames: make([]Frame, 0)}
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
	plan.Pinned = pinned.sorted()

	recording := r.normalizeRecording()
	var captured *recordingOutcome
	if recording != nil {
		captured, err = recording.verify(r.Route, assets, groups, pinned, consumers, r.Policy.Budget)
		if err != nil {
			return plan, err
		}
	}

	startRoute := 0
	liveFrame := 0
	state := make(history)
	replay := Replay{loaded: make(Set)}
	if captured != nil {
		startRoute = captured.nextRoute
		liveFrame = captured.nextLive
		state = captured.history
		replay = captured.replay
		pause := captured.pause.sorted()

		plan.Resident = pause
		plan.StartResident = pause
		plan.PausedResident = pause
		plan.ConfirmedResident = pause
		plan.StartResidency = pause
		plan.NextFrame = liveFrame
		plan.StartFrame = liveFrame
		plan.ResumeFrame = liveFrame
		plan.FromFrame = liveFrame
		plan.Start = liveFrame
		plan.ResumeAt = liveFrame
		plan.PauseFrame = liveFrame - 1
		plan.NextIndex = liveFrame
		plan.StartIndex = liveFrame
		plan.LiveIndex = liveFrame
		plan.NextRoute = startRoute
		plan.StartStep = startRoute
		plan.RouteIndex = startRoute
		plan.NextStep = startRoute
		plan.RecordedFrames = captured.nextLive
		plan.StartBytes = replay.bytes
		plan.PauseBytes = replay.bytes
		remaining := len(r.Route) - startRoute
		meta := &ResumeMetadata{
			AfterFrame:      liveFrame - 1,
			NextFrame:       liveFrame,
			PauseFrame:      liveFrame - 1,
			NextRoute:       startRoute,
			RouteIndex:      startRoute,
			PausedResident:  pause,
			StartResident:   pause,
			RecordedFrames:  captured.nextLive,
			RemainingFrames: remaining,
		}
		plan.Resume = meta
		plan.Checkpoint = meta
		plan.Usage.PeakBytes = replay.bytes
	}

	plan.Crossings = crossings(r.Route, gates)
	if captured != nil {
		remainingCrossings := make([]Crossing, 0)
		for _, c := range plan.Crossings {
			if c.Step >= startRoute {
				remainingCrossings = append(remainingCrossings, c)
			}
		}
		plan.RemainingCrossings = remainingCrossings
	}

	for routeIndex := startRoute; routeIndex < len(r.Route); routeIndex++ {
		v := r.Route[routeIndex]
		if routeIndex > 0 && r.Route[routeIndex-1].Sector != v.Sector {
			state[r.Route[routeIndex-1].Sector] = liveFrame
		}
		recent := state.age(liveFrame, r.Policy.Retain)
		options := prioritize(recent, nearby(v, gates[v.Sector], r.Policy.Lookahead), v.Sector)
		next, decisions, err := admit(groups[v.Sector], pinned, options, groups, assets, r.Policy.Budget)
		if err != nil {
			return plan, fmt.Errorf("visit %d sector %s: %w", routeIndex, v.Sector, err)
		}
		ops := diff(order, replay.loaded, next)
		for _, op := range ops {
			if err := replay.apply(op, assets, consumers, r.Policy.Budget); err != nil {
				return plan, err
			}
		}
		frame := Frame{
			Index:      liveFrame,
			Visit:      v,
			Required:   groups[v.Sector].sorted(),
			Resident:   next.sorted(),
			Bytes:      replay.bytes,
			Operations: ops,
			Decisions:  decisions,
		}
		if captured != nil {
			frame.RouteIndex = routeIndex
			frame.Step = routeIndex
			frame.Route = routeIndex
			frame.LiveIndex = liveFrame
			frame.FrameIndex = liveFrame
		}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)

		if captured != nil {
			// A lookback sector may be preferred only while its complete closure
			// has remained resident. If admission/diff drops it this frame, its
			// lookback life ends; nearby demand can still readmit it later.
			for _, c := range recent {
				if !hasAll(next, groups[c.Sector]) {
					delete(state, c.Sector)
				}
			}
		}
		liveFrame++
	}
	return plan, nil
}
