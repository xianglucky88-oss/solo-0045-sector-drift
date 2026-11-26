package domain

type Usage struct {
	PeakBytes, LoadedBytes, EvictedBytes int64
	Deferred                             int

	// Adaptive-quality aliases. PeakResidentBytes mirrors PeakBytes (which
	// already includes new/old tier coexistence); CoexistPeakBytes is the
	// largest two-tier union. omitempty keeps ordinary plans unchanged.
	PeakResidentBytes int64 `json:"PeakResidentBytes,omitempty"`
	CoexistPeakBytes  int64 `json:"CoexistPeakBytes,omitempty"`
	PeakFrame         *int  `json:"PeakFrame,omitempty"`
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
