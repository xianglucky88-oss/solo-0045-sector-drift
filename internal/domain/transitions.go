package domain

import "fmt"

func verifyRoute(route []Visit, sectors map[string]Sector, gates map[string][]Gate, availability map[string]gateAvailability) error {
	for i, v := range route {
		if sectors[v.Sector].ID == "" || v.Position < 0 || v.Position > 10000 {
			return fmt.Errorf("visit %d is outside the authored world", i)
		}
		if i == 0 || v.Sector == route[i-1].Sector {
			continue
		}
		prior := route[i-1]
		legal := false
		for _, g := range gates[prior.Sector] {
			if g.To != v.Sector || abs(prior.Position-g.At) > g.Reach {
				continue
			}
			if av, ok := availability[g.ID]; ok && (!av.openAt(i-1) || !av.openAt(i)) {
				continue
			}
			legal = true
			break
		}
		if !legal {
			return fmt.Errorf("visit %d crosses without an open gate", i)
		}
	}
	return nil
}
