package domain

type Op struct {
	Action string `json:"Action"`
	Asset  string `json:"Asset"`
	// Bytes is the byte count for a transfer/download instruction. Load and
	// evict operations omit it; the asset record supplies their total size.
	Bytes int64 `json:"Bytes,omitempty"`

	// Alias fields make the same chunk amount available under the names used
	// by different replay documents. They are intentionally omitempty and do
	// not affect classic plans.
	Amount           int64 `json:"Amount,omitempty"`
	TransferBytes    int64 `json:"TransferBytes,omitempty"`
	TransferredBytes int64 `json:"TransferredBytes,omitempty"`
	TotalBytes       int64 `json:"TotalBytes,omitempty"`
	ReceivedBytes    int64 `json:"ReceivedBytes,omitempty"`
	CumulativeBytes  int64 `json:"CumulativeBytes,omitempty"`

	// Vocabulary aliases for replay documents that label the same instruction
	// with a synonym. Action remains the authoritative key.
	Type     string `json:"Type,omitempty"`
	Kind     string `json:"Kind,omitempty"`
	Verb     string `json:"Verb,omitempty"`
	OpName   string `json:"Op,omitempty"`
	Download bool   `json:"Download,omitempty"`
	Transfer bool   `json:"Transfer,omitempty"`
	Activate bool   `json:"Activate,omitempty"`
}

func diff(order []string, old, next Set) []Op {
	out := make([]Op, 0)
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		if old[id] && !next[id] {
			out = append(out, Op{Action: "evict", Asset: id})
		}
	}
	for _, id := range order {
		if next[id] && !old[id] {
			out = append(out, Op{Action: "load", Asset: id})
		}
	}
	return out
}
