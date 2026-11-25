package domain

import "sort"

type Candidate struct {
	Sector, Reason string
	Distance       int
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
func nearby(v Visit, gates []Gate, look int) []Candidate {
	if look == 0 {
		return nil
	}
	var choices []Candidate
	for _, g := range gates {
		d := abs(v.Position - g.At)
		if d <= look {
			choices = append(choices, Candidate{g.To, "gate:" + g.ID, d})
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		a, b := choices[i], choices[j]
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		return a.Reason < b.Reason
	})
	return choices
}
