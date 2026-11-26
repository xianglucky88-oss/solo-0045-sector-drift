package domain

type Usage struct {
	PeakBytes, LoadedBytes, EvictedBytes int64
	Deferred                             int
}

func (u *Usage) record(f Frame, assets map[string]Asset) {
	if f.Bytes > u.PeakBytes {
		u.PeakBytes = f.Bytes
	}
	for _, op := range f.Operations {
		var delta int64
		for _, id := range opUnits(op) {
			delta += assets[id].Bytes
		}
		if op.Action == "load" {
			u.LoadedBytes += delta
		} else {
			u.EvictedBytes += delta
		}
	}
	for _, c := range f.Decisions {
		if !c.Admitted {
			u.Deferred++
		}
	}
}
