package domain

type Op struct{ Action, Asset string }

func diff(order []string, old, next Set) []Op {
	out := make([]Op, 0)
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		if old[id] && !next[id] {
			out = append(out, Op{"evict", id})
		}
	}
	for _, id := range order {
		if next[id] && !old[id] {
			out = append(out, Op{"load", id})
		}
	}
	return out
}
