package domain

import "fmt"

type Replay struct {
	loaded Set
	bytes  int64
}

func (r *Replay) apply(op Op, assets map[string]Asset, consumers map[string][]string, budget int64) error {
	a := assets[op.Asset]
	switch op.Action {
	case "load":
		if r.loaded[a.ID] {
			return fmt.Errorf("double load %s", a.ID)
		}
		for _, d := range a.After {
			if !r.loaded[d] {
				return fmt.Errorf("load before dependency %s", d)
			}
		}
		r.bytes += a.Bytes
		if r.bytes > budget {
			return fmt.Errorf("intermediate working set exceeds budget")
		}
		r.loaded[a.ID] = true
	case "evict":
		if !r.loaded[a.ID] {
			return fmt.Errorf("eviction of absent asset %s", a.ID)
		}
		for _, id := range consumers[a.ID] {
			if r.loaded[id] {
				return fmt.Errorf("eviction would orphan %s", id)
			}
		}
		delete(r.loaded, a.ID)
		r.bytes -= a.Bytes
	default:
		return fmt.Errorf("unknown operation %q", op.Action)
	}
	return nil
}
