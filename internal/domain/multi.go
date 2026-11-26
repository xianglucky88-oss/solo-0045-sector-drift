package domain

import (
	"fmt"
	"sort"
	"strings"
)

// camInfo is one active camera's per-frame candidate view.
type camInfo struct {
	id      string
	visit   Visit
	options []Candidate // prioritize output: unique sectors, recent before gate
	near    []Candidate // distinct near-gate candidates in nearby order
}

type pairKey struct {
	cam    string
	sector string
}

type pairOutcome struct {
	admitted bool
	inc      int64
}

func compileMulti(r Request, tl *timeline) (Plan, error) {
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
	if tl.n < 1 || tl.n > 2048 {
		return plan, fmt.Errorf("route must contain between 1 and 2048 frames")
	}

	// Validate frame occupancy and per-camera continuous-segment traversal.
	for f := 0; f < tl.n; f++ {
		present := tl.present(f)
		if len(present) < 1 || len(present) > 2 {
			return plan, fmt.Errorf("frame %d must have one or two active cameras", f)
		}
		for _, id := range present {
			v := tl.at[id][f]
			if sectors[v.Sector].ID == "" || v.Position < 0 || v.Position > 10000 {
				return plan, fmt.Errorf("frame %d camera %s is outside the authored world", f, id)
			}
			if f > 0 && tl.at[id][f-1] != nil {
				prev := *tl.at[id][f-1]
				if prev.Sector != v.Sector && matchGate(prev, v.Sector, gates) == nil {
					return plan, fmt.Errorf("frame %d camera %s crosses without an open gate", f, id)
				}
			}
		}
	}

	order, err := dependencyOrder(assets)
	if err != nil {
		return plan, err
	}
	consumers := reverseEdges(assets)
	groups := sectorClosures(sectors, assets)
	pinned := closure(r.Policy.Pinned, assets)
	plan.Pinned = pinned.sorted()
	plan.Crossings = multiCrossings(tl, gates)

	histories := make(map[string]history, len(tl.ids))
	waiting := make(map[string]int, len(tl.ids))
	framesPresent := make(map[string]int, len(tl.ids))
	maxWaiting := make(map[string]int, len(tl.ids))
	reportDecisions := make(map[string][]Decision, len(tl.ids))
	reportPrewarms := make(map[string][]Grant, len(tl.ids))
	for _, id := range tl.ids {
		histories[id] = make(history)
	}

	replay := Replay{loaded: make(Set)}
	var grants []Grant

	for f := 0; f < tl.n; f++ {
		present := tl.present(f)
		presentSet := make(Set)
		for _, id := range present {
			presentSet[id] = true
			framesPresent[id]++
		}

		// Cameras off the field lose look-back/prewarm state and their wait
		// restarts from zero on the next join.
		for _, id := range tl.ids {
			if !presentSet[id] {
				histories[id] = make(history)
				waiting[id] = 0
			}
		}

		// Record this frame's departures for continuously present cameras.
		for _, id := range present {
			v := *tl.at[id][f]
			if f > 0 && tl.at[id][f-1] != nil {
				prev := *tl.at[id][f-1]
				if prev.Sector != v.Sector {
					histories[id][prev.Sector] = f
				}
			}
		}

		// Mandatory residency: every active current-sector closure + pinned.
		required := make(Set)
		for _, id := range present {
			required.merge(groups[tl.at[id][f].Sector])
		}
		base := make(Set)
		base.merge(required)
		base.merge(pinned)
		usedBase := byteCost(base, assets)
		if usedBase > r.Policy.Budget {
			return plan, fmt.Errorf("frame %d mandatory residency %d exceeds %d bytes", f, usedBase, r.Policy.Budget)
		}
		free := r.Policy.Budget - usedBase

		infos := make([]camInfo, 0, len(present))
		for _, id := range present {
			v := *tl.at[id][f]
			opts := prioritize(histories[id].age(f, r.Policy.Retain), nearby(v, gates[v.Sector], r.Policy.Lookahead), v.Sector)
			infos = append(infos, camInfo{
				id:      id,
				visit:   v,
				options: opts,
				near:    distinctNear(nearby(v, gates[v.Sector], r.Policy.Lookahead)),
			})
		}

		// Natural decision order: cameras (canonical id order), then each
		// camera's baseline-prioritized options.
		type pair struct {
			cam string
			c   Candidate
		}
		var natural []pair
		owners := make(map[string][]string)
		for _, info := range infos {
			for _, c := range info.options {
				natural = append(natural, pair{info.id, c})
				if !containsString(owners[c.Sector], info.id) {
					owners[c.Sector] = append(owners[c.Sector], info.id)
				}
			}
		}
		for s := range owners {
			sort.Strings(owners[s])
		}

		forced := chooseForced(infos, base, groups, assets, free, waiting)

		// Processing order: arbitration winners first, then every natural
		// pair except the winners (each pair processed exactly once).
		outcomes := make(map[pairKey]pairOutcome)
		resident := make(Set)
		resident.merge(base)
		used := usedBase
		processed := make(map[pairKey]bool)

		process := func(cam string, sector string) {
			k := pairKey{cam, sector}
			if processed[k] {
				return
			}
			processed[k] = true
			cost := incremental(groups[sector], resident, assets)
			ok := cost <= r.Policy.Budget-used
			if ok {
				resident.merge(groups[sector])
				used += cost
			}
			outcomes[k] = pairOutcome{ok, cost}
		}
		for _, k := range forced {
			process(k.cam, k.sector)
		}
		for _, p := range natural {
			process(p.cam, p.c.Sector)
		}

		decisions := make([]Decision, 0, len(natural))
		for _, p := range natural {
			k := pairKey{p.cam, p.c.Sector}
			o := outcomes[k]
			own := append([]string(nil), owners[p.c.Sector]...)
			if len(own) == 0 {
				own = []string{p.cam}
			}
			cand := p.c
			cand.Camera = p.cam
			cand.Shot = p.cam
			d := Decision{
				Candidate: cand,
				Admitted:  o.admitted,
				Increment: o.inc,
				Camera:    p.cam,
				Shot:      p.cam,
				CameraID:  p.cam,
				ShotID:    p.cam,
				Owner:     p.cam,
				Cameras:   own,
				Shots:     own,
			}
			decisions = append(decisions, d)
			reportDecisions[p.cam] = append(reportDecisions[p.cam], d)
			if o.admitted {
				grant := Grant{
					Step: f, Frame: f, Index: f,
					Camera: p.cam, Shot: p.cam, CameraID: p.cam, ShotID: p.cam,
					Sector: p.c.Sector, Reason: p.c.Reason, Distance: p.c.Distance,
					Admitted: true,
				}
				if strings.HasPrefix(p.c.Reason, "gate:") {
					grant.Gate = strings.TrimPrefix(p.c.Reason, "gate:")
					grants = append(grants, grant)
					reportPrewarms[p.cam] = append(reportPrewarms[p.cam], grant)
				}
			}
		}

		ops := diff(order, replay.loaded, resident)
		for _, op := range ops {
			if err := replay.apply(op, assets, consumers, r.Policy.Budget); err != nil {
				return plan, err
			}
		}

		located := make([]LocatedVisit, 0, len(present))
		camFrames := make([]CameraFrame, 0, len(present))
		for _, info := range infos {
			v := info.visit
			located = append(located, LocatedVisit{
				Camera: info.id, Shot: info.id, ID: info.id, CameraID: info.id, ShotID: info.id,
				Sector: v.Sector, Position: v.Position, Present: true,
			})
			readyList := make([]string, 0)
			for _, nc := range info.near {
				if closureWithin(groups[nc.Sector], resident) {
					readyList = append(readyList, nc.Sector)
				}
			}
			ready := len(readyList) > 0
			startWait := waiting[info.id]
			endWait := 0
			if !ready {
				endWait = startWait + 1
			}
			waiting[info.id] = endWait
			if endWait > maxWaiting[info.id] {
				maxWaiting[info.id] = endWait
			}
			var nearCopy []Candidate
			nearCopy = append(nearCopy, info.near...)
			cf := CameraFrame{
				ID: info.id, Camera: info.id, Shot: info.id, CameraID: info.id, ShotID: info.id,
				Sector: v.Sector, Position: v.Position, Present: true, Active: true,
				Visit:   v,
				Waiting: endWait, Wait: endWait, Waited: endWait,
				WaitingFrames: endWait, WaitFrames: endWait, WaitingBefore: startWait,
				Ready: ready, HasReadyCandidate: ready,
				ReadyCandidates: readyList, NearCandidates: nearCopy,
			}
			for _, d := range decisions {
				if d.Camera == info.id {
					cf.Decisions = append(cf.Decisions, d)
				}
			}
			camFrames = append(camFrames, cf)
		}

		frame := Frame{
			Index: f,
			Visit: func() Visit {
				if len(infos) > 0 {
					return infos[0].visit
				}
				return Visit{}
			}(),
			Required:   required.sorted(),
			Resident:   resident.sorted(),
			Bytes:      replay.bytes,
			Operations: ops,
			Decisions:  decisions,
			Visits:     located,
			Cameras:    camFrames,
			Shots:      camFrames,
		}
		plan.Usage.record(frame, assets)
		plan.Frames = append(plan.Frames, frame)
	}

	// Camera reports.
	for _, id := range tl.ids {
		cross := make([]Crossing, 0)
		for _, c := range plan.Crossings {
			if c.Camera == id {
				cross = append(cross, c)
			}
		}
		rep := CameraReport{
			ID: id, Camera: id, Shot: id, CameraID: id, ShotID: id,
			FramesPresent: framesPresent[id], MaxWaiting: maxWaiting[id],
			Crossings: cross,
			Prewarms:  reportPrewarms[id],
			Decisions: reportDecisions[id],
		}
		plan.Cameras = append(plan.Cameras, rep)
		plan.Shots = append(plan.Shots, rep)
		plan.CameraReports = append(plan.CameraReports, rep)
		plan.ShotReports = append(plan.ShotReports, rep)
	}
	plan.Prewarms = grants
	plan.Warmups = grants

	// Canonical crossing order independent of input ordering.
	sort.SliceStable(plan.Crossings, func(i, j int) bool {
		a, b := plan.Crossings[i], plan.Crossings[j]
		if a.Step != b.Step {
			return a.Step < b.Step
		}
		if a.Camera != b.Camera {
			return a.Camera < b.Camera
		}
		return a.Gate < b.Gate
	})
	// Re-sync report crossings after the global sort.
	for i := range plan.CameraReports {
		id := plan.CameraReports[i].ID
		var cross []Crossing
		for _, c := range plan.Crossings {
			if c.Camera == id {
				cross = append(cross, c)
			}
		}
		plan.CameraReports[i].Crossings = cross
		plan.Cameras[i].Crossings = cross
		plan.Shots[i].Crossings = cross
		plan.ShotReports[i].Crossings = cross
	}
	return plan, nil
}

// chooseForced implements the two-level guarantee:
//  1. if any combination makes every active camera ready, take the
//     lexicographically first feasible combination;
//  2. otherwise serve the single highest-priority serviceable camera.
func chooseForced(infos []camInfo, base Set, groups map[string]Set, assets map[string]Asset, free int64, waiting map[string]int) []pairKey {
	type choice struct {
		cam    string
		sector string // empty means "already ready via mandatory residency"
	}
	choiceLists := make([][]choice, 0, len(infos))
	for _, info := range infos {
		var list []choice
		baseReady := false
		for _, nc := range info.near {
			if closureWithin(groups[nc.Sector], base) {
				baseReady = true
				break
			}
		}
		if baseReady {
			list = append(list, choice{info.id, ""})
		} else {
			for _, nc := range info.near {
				list = append(list, choice{info.id, nc.Sector})
			}
		}
		if len(list) == 0 {
			// Camera with no satisfiable choice: no all-ready combination.
			choiceLists = append(choiceLists, nil)
		} else {
			choiceLists = append(choiceLists, list)
		}
	}

	// Any empty choice list makes the all-ready search impossible.
	possible := true
	for _, l := range choiceLists {
		if len(l) == 0 {
			possible = false
		}
	}
	if possible {
		// At most two active cameras per frame. Nested search with
		// incremental costing correctly accounts for shared closures.
		var best []choice
		if len(choiceLists) == 1 {
			for _, x := range choiceLists[0] {
				var c int64
				if x.sector != "" {
					c = incremental(groups[x.sector], base, assets)
				}
				if c <= free {
					best = []choice{x}
					break
				}
			}
		} else if len(choiceLists) == 2 {
			for _, x := range choiceLists[0] {
				afterX := base
				cx := int64(0)
				if x.sector != "" {
					cx = incremental(groups[x.sector], base, assets)
					if cx > free {
						continue
					}
					afterX = make(Set)
					afterX.merge(base)
					afterX.merge(groups[x.sector])
				}
				for _, y := range choiceLists[1] {
					cy := int64(0)
					if y.sector != "" {
						cy = incremental(groups[y.sector], afterX, assets)
					}
					if cx+cy <= free {
						best = []choice{x, y}
						break
					}
				}
				if best != nil {
					break
				}
			}
		}
		if best != nil {
			var forced []pairKey
			for _, ch := range best {
				if ch.sector != "" {
					forced = append(forced, pairKey{ch.cam, ch.sector})
				}
			}
			return forced
		}
	}

	// Only one camera can be cared for. Serviceable = has an individually
	// affordable near-gate candidate and is not already mandatory-ready.
	type cand struct {
		key pairKey
		w   int
		id  string
	}
	var serviceable []cand
	for _, info := range infos {
		baseReady := false
		for _, nc := range info.near {
			if closureWithin(groups[nc.Sector], base) {
				baseReady = true
				break
			}
		}
		if baseReady {
			// Already made ready by mandatory residency; the scarce
			// guaranteed admission is meant for a camera still waiting.
			continue
		}
		for _, nc := range info.near {
			if incremental(groups[nc.Sector], base, assets) <= free {
				serviceable = append(serviceable, cand{pairKey{info.id, nc.Sector}, waiting[info.id], info.id})
				break // first priority-ordered affordable candidate per camera
			}
		}
	}
	if len(serviceable) == 0 {
		return nil
	}
	sort.SliceStable(serviceable, func(i, j int) bool {
		if serviceable[i].w != serviceable[j].w {
			return serviceable[i].w > serviceable[j].w // longer wait first
		}
		return serviceable[i].id < serviceable[j].id // deterministic tie
	})
	return []pairKey{serviceable[0].key}
}

func distinctNear(in []Candidate) []Candidate {
	seen := make(Set)
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if seen[c.Sector] {
			continue
		}
		seen[c.Sector] = true
		out = append(out, c)
	}
	return out
}

func closureWithin(closure, resident Set) bool {
	for id := range closure {
		if !resident[id] {
			return false
		}
	}
	return true
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func matchGate(prev Visit, toSector string, gates map[string][]Gate) *Gate {
	for i := range gates[prev.Sector] {
		g := gates[prev.Sector][i]
		if g.To == toSector && abs(prev.Position-g.At) <= g.Reach {
			return &g
		}
	}
	return nil
}

func multiCrossings(tl *timeline, gates map[string][]Gate) []Crossing {
	out := make([]Crossing, 0)
	for _, id := range tl.ids {
		for f := 1; f < tl.n; f++ {
			cur := tl.at[id][f]
			prev := tl.at[id][f-1]
			if cur == nil || prev == nil || prev.Sector == cur.Sector {
				continue
			}
			g := matchGate(*prev, cur.Sector, gates)
			if g == nil {
				continue
			}
			out = append(out, Crossing{
				Step: f, Gate: g.ID, From: prev.Sector, To: cur.Sector,
				Camera: id, Shot: id, CameraID: id, ShotID: id, Owner: id, By: id,
			})
		}
	}
	return out
}
