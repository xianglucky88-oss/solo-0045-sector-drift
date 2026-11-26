package domain

import "fmt"

type Replay struct {
	loaded Set
	bytes  int64
	gi     *groupIndex
}

func NewReplay(gi *groupIndex) *Replay {
	return &Replay{loaded: make(Set), gi: gi}
}

// Loaded exposes the current resident set (sorted copy).
func (r *Replay) Loaded() Set  { return r.loaded }
func (r *Replay) Bytes() int64 { return r.bytes }

func (r *Replay) apply(op Op, assets map[string]Asset, consumers map[string][]string, budget int64) error {
	ids := opUnits(op)
	if len(ids) == 0 {
		return fmt.Errorf("operation %q names no asset", op.Action)
	}
	inOp := toSet(ids)
	switch op.Action {
	case "load":
		addBytes := int64(0)
		for _, id := range ids {
			a, ok := assets[id]
			if !ok {
				return fmt.Errorf("operation references unknown asset %q", id)
			}
			if r.loaded[id] {
				return fmt.Errorf("double load %s", id)
			}
			addBytes += a.Bytes
		}
		// External dependencies must already be resident. Dependencies satisfied
		// by another member of the same atomic op are registered together, so
		// intra-group mutual references are accepted.
		for _, id := range ids {
			for _, d := range assets[id].After {
				if inOp[d] {
					continue
				}
				if !r.loaded[d] {
					return fmt.Errorf("load before dependency %s", d)
				}
			}
		}
		if r.bytes+addBytes > budget {
			return fmt.Errorf("intermediate working set exceeds budget")
		}
		for _, id := range ids {
			r.loaded[id] = true
		}
		r.bytes += addBytes
	case "evict":
		removeBytes := int64(0)
		for _, id := range ids {
			a, ok := assets[id]
			if !ok {
				return fmt.Errorf("operation references unknown asset %q", id)
			}
			if !r.loaded[id] {
				return fmt.Errorf("eviction of absent asset %s", id)
			}
			removeBytes += a.Bytes
		}
		// No resident consumer outside this atomic op may still reference a
		// member. Consumers inside the same op leave together.
		for _, id := range ids {
			for _, cid := range consumers[id] {
				if inOp[cid] {
					continue
				}
				if r.loaded[cid] {
					return fmt.Errorf("eviction would orphan %s", cid)
				}
			}
		}
		for _, id := range ids {
			delete(r.loaded, id)
		}
		r.bytes -= removeBytes
	default:
		return fmt.Errorf("unknown operation %q", op.Action)
	}
	return nil
}
