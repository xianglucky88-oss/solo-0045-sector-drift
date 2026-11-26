package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Route-search inputs. These are populated by UnmarshalJSON from the
	// canonical wire names; a wider set of aliases and equivalent shapes is
	// accepted there as well. An explicitly authored Route always compiles
	// through the original path.
	Signposts []Signpost
	Moves     []Move
	Start     string
	End       string
	MaxMoves  int

	// In-process aliases for the same search inputs. JSON callers should use
	// the canonical names; these exist for source-level compatibility.
	Waypoints []Signpost
	Markers   []Signpost
	Edges     []Move
	Goal      string
	MaxSteps  int

	// RouteRefs is an explicitly authored route expressed as signpost
	// references; it is resolved against Signposts before compiling.
	RouteRefs []string

	raw map[string]json.RawMessage
}

type Sector struct {
	ID     string
	Assets []string
}
type Asset struct {
	ID    string
	Bytes int64
	After []string
}
type Gate struct {
	ID, From, To string
	At, Reach    int
	OneWay       bool
}
type Visit struct {
	Sector   string
	Position int

	// Optional signpost provenance for route-search plans. It is omitted for
	// legacy visits and accepted (but not required) on input.
	ID         string `json:"ID,omitempty"`
	Signpost   string `json:"Signpost,omitempty"`
	SignpostID string `json:"SignpostID,omitempty"`
	Ref        string `json:"Ref,omitempty"`
	Name       string `json:"Name,omitempty"`
}
type Policy struct {
	Budget            int64
	Lookahead, Retain int
	Pinned            []string

	// MaxMoves may alternatively be carried by the policy block.
	MaxMoves      *int `json:"MaxMoves,omitempty"`
	MaxMovesSnake *int `json:"max_moves,omitempty"`
	MaxSteps      *int `json:"MaxSteps,omitempty"`
}

// Signpost is a named Visit: a position inside a sector.
type Signpost struct {
	ID       string `json:"ID"`
	Sector   string `json:"Sector"`
	Position int    `json:"Position"`
}

// Move is one allowed directed transition between two signposts. Gate is
// optional; when absent the gate is inferred from sector and departure
// position using the established rule.
type Move struct {
	From string `json:"From"`
	To   string `json:"To"`
	Gate string `json:"Gate,omitempty"`
}

// normKey folds case and separators so MaxMoves, max_moves and max-moves all
// compare equal.
func normKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '_', '-', ' ', '.':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func strictDecode(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func rawText(raw json.RawMessage) string {
	return strings.TrimSpace(string(raw))
}

func rawPresent(raw json.RawMessage) bool {
	t := rawText(raw)
	return t != "" && t != "null"
}

// UnmarshalJSON keeps the legacy strict contract (unknown fields are
// rejected) while accepting the route-search vocabulary under a broad set of
// names and equivalent shapes.
func (r *Request) UnmarshalJSON(data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return err
	}
	*r = Request{}
	r.raw = top

	for key, raw := range top {
		nk := normKey(key)
		switch nk {
		case "sectors":
			if err := strictDecode(raw, &r.Sectors); err != nil {
				return fmt.Errorf("field %s: %w", key, err)
			}
		case "assets":
			if err := strictDecode(raw, &r.Assets); err != nil {
				return fmt.Errorf("field %s: %w", key, err)
			}
		case "gates":
			if err := strictDecode(raw, &r.Gates); err != nil {
				return fmt.Errorf("field %s: %w", key, err)
			}
		case "policy":
			if err := strictDecode(raw, &r.Policy); err != nil {
				return fmt.Errorf("field %s: %w", key, err)
			}
		case "route":
			visits, refs, err := decodeRouteWire(raw)
			if err != nil {
				return err
			}
			r.Route = visits
			r.RouteRefs = refs
		default:
			// Search vocabulary is normalized later from r.raw; here we only
			// enforce that the key is a recognized alias or an unmistakable
			// search-shaped value.
			if !searchKeyKnown(nk) && classifyFallback(key, raw) == "" {
				return fmt.Errorf("json: unknown field %q", key)
			}
		}
	}
	return nil
}

// decodeRouteWire accepts the legacy []Visit document as well as a list of
// signpost references.
func decodeRouteWire(raw json.RawMessage) ([]Visit, []string, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		// Not an array: preserve the legacy type error.
		var visits []Visit
		if derr := strictDecode(raw, &visits); derr != nil {
			return nil, nil, derr
		}
		return visits, nil, nil
	}
	if len(items) == 0 {
		return []Visit{}, nil, nil
	}
	first := rawText(items[0])
	if strings.HasPrefix(first, "{") {
		var visits []Visit
		if err := strictDecode(raw, &visits); err != nil {
			return nil, nil, err
		}
		return visits, nil, nil
	}
	refs := make([]string, 0, len(items))
	for _, it := range items {
		var ref any
		if err := json.Unmarshal(it, &ref); err != nil {
			return nil, nil, err
		}
		refs = append(refs, fmt.Sprint(ref))
	}
	return nil, refs, nil
}
