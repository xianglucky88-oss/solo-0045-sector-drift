package domain

import (
	"fmt"
	"sort"
)

func connect(src []Gate, sectors map[string]Sector) (map[string][]Gate, error) {
	out := make(map[string][]Gate)
	seen := make(Set)
	for _, g := range src {
		if !label.MatchString(g.ID) || seen[g.ID] || g.From == g.To || g.At < 0 || g.At > 10000 || g.Reach < 0 || g.Reach > 10000 {
			return nil, fmt.Errorf("invalid gate %q", g.ID)
		}
		if sectors[g.From].ID == "" || sectors[g.To].ID == "" {
			return nil, fmt.Errorf("gate %s references unknown sector", g.ID)
		}
		seen[g.ID] = true
		out[g.From] = append(out[g.From], g)
		if !g.OneWay {
			g.From, g.To = g.To, g.From
			out[g.From] = append(out[g.From], g)
		}
	}
	for _, gs := range out {
		sort.Slice(gs, func(i, j int) bool { return gs[i].ID < gs[j].ID })
	}
	return out, nil
}
