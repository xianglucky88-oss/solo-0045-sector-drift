package domain

type Usage struct {
	PeakBytes, LoadedBytes, EvictedBytes int64
	Deferred                             int
	MovedBytes                           int64 `json:",omitempty"`
}

func (u *Usage) record(f Frame, assets map[string]Asset) {
	if f.Bytes > u.PeakBytes {
		u.PeakBytes = f.Bytes
	}
	for _, op := range f.Operations {
		switch op.Action {
		case "load":
			u.LoadedBytes += assets[op.Asset].Bytes
		case "move":
			u.MovedBytes += assets[op.Asset].Bytes
		default:
			u.EvictedBytes += assets[op.Asset].Bytes
		}
	}
	for _, c := range f.Decisions {
		if !c.Admitted {
			u.Deferred++
		}
	}
}
