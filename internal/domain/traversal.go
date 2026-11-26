package domain

type Crossing struct {
	Step           int
	Gate, From, To string
}

func crossings(route []Visit, gates map[string][]Gate, availability map[string]gateAvailability) []Crossing {
	out := make([]Crossing, 0)
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		if a.Sector == b.Sector {
			continue
		}
		for _, g := range gates[a.Sector] {
			if g.To != b.Sector || abs(a.Position-g.At) > g.Reach {
				continue
			}
			if av, ok := availability[g.ID]; ok && (!av.openAt(i-1) || !av.openAt(i)) {
				continue
			}
			out = append(out, Crossing{i, g.ID, a.Sector, b.Sector})
			break
		}
	}
	return out
}
