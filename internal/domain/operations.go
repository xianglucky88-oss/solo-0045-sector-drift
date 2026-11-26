package domain

type Op struct {
	Action string `json:"Action"`
	Asset  string `json:"Asset,omitempty"`

	// Group marks an atomic resource-group operation. Assets and Members both
	// carry the full member set; Batch repeats the correlation identifier so
	// engines that batch flat operations can group them.
	Group       string   `json:"Group,omitempty"`
	Batch       string   `json:"Batch,omitempty"`
	Assets      []string `json:"Assets,omitempty"`
	AssetIDs    []string `json:"AssetIDs,omitempty"`
	Members     []string `json:"Members,omitempty"`
	IDs         []string `json:"IDs,omitempty"`
	Targets     []string `json:"Targets,omitempty"`
	GroupAssets []string `json:"GroupAssets,omitempty"`
	Atomic      bool     `json:"Atomic,omitempty"`
	Kind        string   `json:"Kind,omitempty"`
	Type        string   `json:"Type,omitempty"`
	OpName      string   `json:"Op,omitempty"`
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

// diffUnits emits evictions in reverse dependency order followed by loads in
// dependency order, at the granularity of atomic units.
func diffUnits(order []Unit, old, next Set) []Op {
	out := make([]Op, 0)
	present := func(s Set, u Unit) bool { return s[u.Members[0]] }
	for i := len(order) - 1; i >= 0; i-- {
		u := order[i]
		if present(old, u) && !present(next, u) {
			out = append(out, unitOp("evict", u))
		}
	}
	for _, u := range order {
		if present(next, u) && !present(old, u) {
			out = append(out, unitOp("load", u))
		}
	}
	return out
}

func unitOp(action string, u Unit) Op {
	if u.GroupID == "" {
		return Op{Action: action, Asset: u.Members[0]}
	}
	members := append([]string(nil), u.Members...)
	kind := action + "-group"
	return Op{
		Action:      action,
		Asset:       members[0],
		Group:       u.GroupID,
		Batch:       u.GroupID,
		Assets:      append([]string(nil), members...),
		AssetIDs:    append([]string(nil), members...),
		Members:     append([]string(nil), members...),
		IDs:         append([]string(nil), members...),
		Targets:     append([]string(nil), members...),
		GroupAssets: append([]string(nil), members...),
		Atomic:      true,
		Kind:        kind,
		Type:        kind,
		OpName:      kind,
	}
}

// inverse returns the compensating operation for an applied op.
func inverse(op Op) Op {
	out := op
	if op.Action == "load" {
		out.Action = "evict"
		out.Kind = swapKind(op.Kind, "load-group", "evict-group")
	} else {
		out.Action = "load"
		out.Kind = swapKind(op.Kind, "evict-group", "load-group")
	}
	return out
}

func swapKind(kind, from, to string) string {
	if kind == from {
		return to
	}
	return kind
}

// flatOperations expands group ops into one op per member while preserving the
// batch correlation. Ungrouped ops are returned unchanged.
func flatOperations(ops []Op) []Op {
	out := make([]Op, 0, len(ops))
	for _, op := range ops {
		ids := opUnits(op)
		if len(ids) <= 1 {
			out = append(out, op)
			continue
		}
		for _, id := range ids {
			one := op
			one.Asset = id
			one.Assets = append([]string(nil), op.Assets...)
			one.Members = append([]string(nil), op.Members...)
			out = append(out, one)
		}
	}
	return out
}
