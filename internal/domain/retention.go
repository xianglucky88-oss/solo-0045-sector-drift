package domain

type history map[string]int

func (h history) addStep(route []Visit, i int) {
	if i > 0 && route[i-1].Sector != route[i].Sector {
		h[route[i-1].Sector] = i
	}
}
func (h history) age(step, keep int) []Candidate {
	var out []Candidate
	for s, left := range h {
		delta := step - left
		if delta >= keep {
			delete(h, s)
			continue
		}
		out = append(out, Candidate{Sector: s, Reason: "recent", Distance: delta})
	}
	return out
}
