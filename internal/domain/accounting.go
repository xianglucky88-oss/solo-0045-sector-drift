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
		if op.Action == "load" {
			u.LoadedBytes += assets[op.Asset].Bytes
		} else {
			u.EvictedBytes += assets[op.Asset].Bytes
		}
	}
	for _, c := range f.Decisions {
		if !c.Admitted {
			u.Deferred++
		}
	}
}
