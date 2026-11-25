package domain

import "sort"

func prioritize(recent, near []Candidate, current string) []Candidate {
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].Distance != recent[j].Distance {
			return recent[i].Distance < recent[j].Distance
		}
		return recent[i].Sector < recent[j].Sector
	})
	seen := Set{current: true}
	out := make([]Candidate, 0, len(recent)+len(near))
	for _, c := range append(recent, near...) {
		if seen[c.Sector] {
			continue
		}
		out = append(out, c)
		seen[c.Sector] = true
	}
	return out
}
