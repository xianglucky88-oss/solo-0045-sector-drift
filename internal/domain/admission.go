package domain

import "fmt"

type Decision struct {
	Candidate Candidate
	Admitted  bool
	Increment int64
}

func admit(active, pinned Set, choices []Candidate, groups map[string]Set, assets map[string]Asset, budget int64) (Set, []Decision, error) {
	core := make(Set)
	core.merge(active)
	core.merge(pinned)
	return admitCore(core, choices, groups, assets, budget)
}

// admitCore allocates optional sector closures on top of an arbitrary mandatory
// core (current sector closure, pinned closure and active lease closures).
func admitCore(core Set, choices []Candidate, groups map[string]Set, assets map[string]Asset, budget int64) (Set, []Decision, error) {
	resident := make(Set)
	resident.merge(core)
	used := byteCost(resident, assets)
	if used > budget {
		return nil, nil, fmt.Errorf("mandatory residency %d exceeds %d bytes", used, budget)
	}
	decisions := make([]Decision, 0, len(choices))
	for _, c := range choices {
		cost := incremental(groups[c.Sector], resident, assets)
		allowed := cost <= budget-used
		if allowed {
			resident.merge(groups[c.Sector])
			used += cost
		}
		decisions = append(decisions, Decision{c, allowed, cost})
	}
	return resident, decisions, nil
}
