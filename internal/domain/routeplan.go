package domain

import "fmt"

func compileSearch(r Request, s Search) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget}

	e, err := buildEngine(r, s)
	if err != nil {
		return plan, err
	}
	startIx, ok1 := e.spIndex[s.Start]
	endIx, ok2 := e.spIndex[s.End]
	if !ok1 || !ok2 {
		return plan, fmt.Errorf("start or end signpost is not defined")
	}

	answer, err := e.searchRoute(startIx, endIx, s.MaxMoves)
	if err != nil {
		return plan, err
	}
	labels := chain(answer)

	visits := make([]Visit, len(labels))
	ids := make([]string, len(labels))
	details := make([]Signpost, len(labels))
	steps := make([]RouteStep, 0, len(labels)-1)
	crossingRecords := make([]Crossing, 0)
	for i, l := range labels {
		sp := e.signposts[l.key.sp]
		ids[i] = sp.ID
		visits[i] = Visit{
			Sector: sp.Sector, Position: sp.Position,
			ID: sp.ID, Signpost: sp.ID, SignpostID: sp.ID, Ref: sp.ID, Name: sp.ID,
		}
		details[i] = sp
		if i > 0 && l.via != nil {
			prev := e.signposts[labels[i-1].key.sp]
			step := RouteStep{
				Step:       i,
				From:       prev.ID,
				To:         sp.ID,
				Crossing:   l.via.crossing,
				FromSector: prev.Sector,
				ToSector:   sp.Sector,
			}
			if l.via.crossing {
				step.Gate = l.via.gateID
				crossingRecords = append(crossingRecords, Crossing{
					Step: i, Gate: l.via.gateID, From: prev.Sector, To: sp.Sector,
				})
			}
			steps = append(steps, step)
		}
	}

	pinned := closure(r.Policy.Pinned, e.assets)
	plan.Pinned = pinned.sorted()
	plan.Crossings = crossingRecords

	consumers := reverseEdges(e.assets)
	replay := Replay{loaded: make(Set)}
	for i, l := range labels {
		resident := e.residencySet(l.key.b)
		ops := diff(e.order, replay.loaded, resident)
		for _, op := range ops {
			if err := replay.apply(op, e.assets, consumers, r.Policy.Budget); err != nil {
				return plan, fmt.Errorf("frame %d: %w", i, err)
			}
		}
		if replay.bytes != e.bitsCost(l.key.b) {
			return plan, fmt.Errorf("frame %d resident byte accounting mismatch", i)
		}
		id := ids[i]
		frame := Frame{
			Index:       i,
			Visit:       visits[i],
			Required:    e.reqSet[l.key.sp].sorted(),
			Resident:    resident.sorted(),
			Bytes:       replay.bytes,
			Operations:  ops,
			Decisions:   []Decision{},
			ID:          id,
			Signpost:    id,
			SignpostID:  id,
			SignpostRef: id,
			Waypoint:    id,
			Marker:      id,
			Name:        id,
			Ref:         id,
			Node:        id,
		}
		plan.Usage.record(frame, e.assets)
		plan.Frames = append(plan.Frames, frame)
	}

	if plan.Usage.LoadedBytes != answer.cost {
		return plan, fmt.Errorf("route load total %d does not match search cost %d", plan.Usage.LoadedBytes, answer.cost)
	}

	plan.Signposts = ids
	plan.Waypoints = ids
	plan.RouteSignposts = ids
	plan.SelectedRoute = ids
	plan.ChosenRoute = ids
	plan.SignpostRoute = ids
	plan.Path = ids
	plan.RouteIDs = ids
	plan.Route = ids
	plan.SelectedSignposts = details
	plan.SignpostDetails = details
	plan.VisitedSignposts = details
	plan.ChosenSignposts = details
	plan.RouteVisits = visits
	plan.Visits = visits
	plan.Steps = steps
	plan.RouteMoves = steps
	plan.UsedMoves = steps
	plan.MoveLog = steps
	plan.LoadedBytes = plan.Usage.LoadedBytes
	plan.TotalLoadedBytes = plan.Usage.LoadedBytes
	plan.GrossLoadedBytes = plan.Usage.LoadedBytes
	plan.SelectedMovesCount = len(labels) - 1
	return plan, nil
}
