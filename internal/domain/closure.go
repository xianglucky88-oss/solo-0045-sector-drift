package domain

func closure(roots []string, assets map[string]Asset) Set {
	return expandClosure(roots, assets, nil)
}

// expandClosure computes the dependency closure of roots. Whenever a reached
// asset belongs to a declared group, every member of that group is pulled in
// together with the dependencies of every member.
func expandClosure(roots []string, assets map[string]Asset, gi *groupIndex) Set {
	result := make(Set)
	pending := append([]string(nil), roots...)
	for len(pending) > 0 {
		idx := len(pending) - 1
		id := pending[idx]
		pending = pending[:idx]
		u := gi.unitOf(id)
		for _, m := range u.Members {
			if result[m] {
				continue
			}
			result[m] = true
			pending = append(pending, assets[m].After...)
		}
	}
	return result
}

func sectorClosures(sectors map[string]Sector, assets map[string]Asset) map[string]Set {
	return sectorExpansions(sectors, assets, nil)
}

func sectorExpansions(sectors map[string]Sector, assets map[string]Asset, gi *groupIndex) map[string]Set {
	out := make(map[string]Set, len(sectors))
	for id, sector := range sectors {
		out[id] = expandClosure(sector.Assets, assets, gi)
	}
	return out
}
