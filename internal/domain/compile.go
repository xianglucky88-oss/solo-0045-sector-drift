package domain

import (
	"encoding/json"
	"fmt"
)

func compile(r Request) (Plan, error) {
	plan, err := compileCore(r)
	if err != nil {
		return plan, err
	}
	if frame, member, requested, derr := r.DrillTarget(); derr != nil {
		return plan, derr
	} else if requested {
		assets, aerr := indexAssets(r.Assets)
		if aerr != nil {
			return plan, aerr
		}
		gi, gerr := indexGroups(r.allGroups(), assets)
		if gerr != nil {
			return plan, gerr
		}
		report, derr := runDrill(&plan, frame, member, assets, reverseEdges(assets), gi, r.Policy.Budget)
		if derr != nil {
			return plan, derr
		}
		attachDrill(&plan, report)
	}
	return plan, nil
}

func compileCore(r Request) (Plan, error) {
	plan := Plan{Schema: "sector-stream/v1", Budget: r.Policy.Budget}
	if err := validate(r); err != nil {
		return plan, err
	}
	assets, err := indexAssets(r.Assets)
	if err != nil {
		return plan, err
	}
	gi, err := indexGroups(r.allGroups(), assets)
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
	units, err := unitOrder(assets, gi)
	if err != nil {
		return plan, err
	}
	// Keep the legacy whole-graph ordering validated as well for ungroupless
	// maps; unitOrder is the group-aware authority.
	if len(gi.ids) == 0 {
		if _, err := dependencyOrder(assets); err != nil {
			return plan, err
		}
	}
	consumers := reverseEdges(assets)
	sectorSets := sectorExpansions(sectors, assets, gi)
	pinned := expandClosure(r.Policy.Pinned, assets, gi)
	plan.Pinned = pinned.sorted()
	if infos := gi.info(assets); len(infos) > 0 {
		plan.Groups = infos
		plan.ResourceGroups = infos
		byID := make(map[string]GroupInfo, len(infos))
		for _, g := range infos {
			byID[g.ID] = g
		}
		plan.GroupMap = byID
		plan.GroupsByID = byID
	}
	plan.Crossings = crossings(r.Route, gates)
	state := make(history)
	replay := &Replay{loaded: make(Set), gi: gi}
	for i, v := range r.Route {
		state.addStep(r.Route, i)
		options := prioritize(state.age(i, r.Policy.Retain), nearby(v, gates[v.Sector], r.Policy.Lookahead), v.Sector)
		next, decisions, err := admitUnits(sectorSets[v.Sector], pinned, options, sectorSets, assets, gi, r.Policy.Budget)
		if err != nil {
			return plan, fmt.Errorf("visit %d sector %s: %w", i, v.Sector, err)
		}
		ops := diffUnits(units, replay.loaded, next)
		for _, op := range ops {
			if err := replay.apply(op, assets, consumers, r.Policy.Budget); err != nil {
				return plan, err
			}
		}
		groupOps := groupOnly(ops)
		frame := Frame{
			Index:           i,
			Visit:           v,
			Required:        sectorSets[v.Sector].sorted(),
			Resident:        next.sorted(),
			Bytes:           replay.bytes,
			Operations:      ops,
			Decisions:       decisions,
			GroupOperations: groupOps,
			LoadedGroups:    groupsInSet(next, gi),
			ResidentGroups:  groupsInSet(next, gi),
			RequiredGroups:  groupsInSet(sectorSets[v.Sector], gi),
		}
		// Only emit the flat per-member expansion when atomic group ops exist,
		// keeping undeclared plans byte-identical to the original contract.
		if len(groupOps) > 0 {
			frame.FlatOperations = flatOperations(ops)
			frame.AssetOperations = flatOperations(ops)
			frame.Batches = ops
			sets := make([][]string, 0, len(groupOps))
			for _, op := range groupOps {
				sets = append(sets, append([]string(nil), op.Assets...))
			}
			frame.BatchSets = sets
		}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)
	}

	return plan, nil
}

// DrillCompiled compiles r and replays an explicitly targeted failure,
// returning the encoded plan (with report attached) and the report itself.
func DrillCompiled(r Request, frame int, asset string) ([]byte, *DrillReport, error) {
	plan, err := compileCore(r)
	if err != nil {
		return nil, nil, err
	}
	assets, err := indexAssets(r.Assets)
	if err != nil {
		return nil, nil, err
	}
	gi, err := indexGroups(r.allGroups(), assets)
	if err != nil {
		return nil, nil, err
	}
	report, err := runDrill(&plan, frame, asset, assets, reverseEdges(assets), gi, r.Policy.Budget)
	if err != nil {
		return nil, nil, err
	}
	attachDrill(&plan, report)
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, report, nil
}

func groupOnly(ops []Op) []Op {
	out := make([]Op, 0)
	for _, op := range ops {
		if op.Group != "" || len(op.Assets) > 1 {
			out = append(out, op)
		}
	}
	return out
}

func attachDrill(plan *Plan, report *DrillReport) {
	plan.DrillReport = report
	plan.Drill = report
	plan.DrillResult = report
	plan.DrillReport2 = report
	plan.Report = report
	plan.Rehearsal = report
	plan.Fault = report
	plan.Failure = report
	plan.FaultDrill = report
	plan.FailureDrill = report
	plan.Replay = report
	plan.ReplayReport = report
}
