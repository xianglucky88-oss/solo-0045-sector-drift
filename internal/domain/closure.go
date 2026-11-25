package domain

func closure(roots []string, assets map[string]Asset) Set {
	result := make(Set)
	pending := append([]string(nil), roots...)
	for len(pending) > 0 {
		idx := len(pending) - 1
		id := pending[idx]
		pending = pending[:idx]
		if result[id] {
			continue
		}
		result[id] = true
		pending = append(pending, assets[id].After...)
	}
	return result
}
func sectorClosures(sectors map[string]Sector, assets map[string]Asset) map[string]Set {
	out := make(map[string]Set, len(sectors))
	for id, sector := range sectors {
		out[id] = closure(sector.Assets, assets)
	}
	return out
}
