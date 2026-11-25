package domain

import "sort"

type Set map[string]bool

func (s Set) merge(from Set) {
	for id := range from {
		s[id] = true
	}
}
func (s Set) sorted() []string {
	out := make([]string, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
