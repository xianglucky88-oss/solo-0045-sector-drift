package domain

import "encoding/json"

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// aliases carries additional JSON keys used by adaptive-quality plans.
	// It is never serialised under its own name.
	aliases map[string]any `json:"-"`
}

// MarshalJSON keeps the baseline frame document identical when no quality
// aliases exist, and merges the compatibility keys otherwise.
func (f Frame) MarshalJSON() ([]byte, error) {
	type plain Frame
	b, err := json.Marshal(plain(f))
	if err != nil {
		return nil, err
	}
	if len(f.aliases) == 0 {
		return b, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range f.aliases {
		m[k] = v
	}
	return json.Marshal(m)
}

// setAlias stores one compatibility JSON key for the frame.
func (f *Frame) setAlias(key string, value any) {
	if f.aliases == nil {
		f.aliases = make(map[string]any)
	}
	f.aliases[key] = value
}
