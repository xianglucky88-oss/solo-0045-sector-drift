package domain

type Decision struct {
	Candidate Candidate
	Admitted  bool
	Increment int64

	// Groups pulled in by this candidate, when any.
	Groups []string `json:"Groups,omitempty"`
}

func admit(active, pinned Set, choices []Candidate, groups map[string]Set, assets map[string]Asset, budget int64) (Set, []Decision, error) {
	return admitUnits(active, pinned, choices, groups, assets, nil, budget)
}

func admitUnits(active, pinned Set, choices []Candidate, groups map[string]Set, assets map[string]Asset, gi *groupIndex, budget int64) (Set, []Decision, error) {
	resident := make(Set)
	resident.merge(active)
	resident.merge(pinned)
	used := byteCost(resident, assets)
	if used > budget {
		return nil, nil, mandatoryBlock(resident, assets, gi, used, budget)
	}
	decisions := make([]Decision, 0, len(choices))
	for _, c := range choices {
		cost := incremental(groups[c.Sector], resident, assets)
		allowed := cost <= budget-used
		decision := Decision{Candidate: c, Admitted: allowed, Increment: cost,
			Groups: groupsInSet(groups[c.Sector], gi)}
		if !allowed {
			decision.Groups = append(decision.Groups, blockedGroups(groups[c.Sector], resident, gi)...)
			decision.Groups = uniqueSorted(decision.Groups)
		}
		if allowed {
			resident.merge(groups[c.Sector])
			used += cost
		}
		decisions = append(decisions, decision)
	}
	return resident, decisions, nil
}

func groupsInSet(s Set, gi *groupIndex) []string {
	if gi == nil {
		return nil
	}
	gids := make(Set)
	for id := range s {
		if g := gi.byAsset[id]; g != "" {
			gids[g] = true
		}
	}
	return gids.sorted()
}

// blockedGroups names groups whose not-yet-resident bytes drive a rejection.
func blockedGroups(candidate, resident Set, gi *groupIndex) []string {
	if gi == nil {
		return nil
	}
	gids := make(Set)
	for id := range candidate {
		if resident[id] {
			continue
		}
		if g := gi.byAsset[id]; g != "" {
			gids[g] = true
		}
	}
	return gids.sorted()
}

func uniqueSorted(in []string) []string {
	return toSet(in).sorted()
}
