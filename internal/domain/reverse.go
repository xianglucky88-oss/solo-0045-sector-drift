package domain

import "sort"

func reverseEdges(assets map[string]Asset) map[string][]string {
	out := make(map[string][]string)
	for id, a := range assets {
		for _, d := range a.After {
			out[d] = append(out[d], id)
		}
	}
	for _, ids := range out {
		sort.Strings(ids)
	}
	return out
}
