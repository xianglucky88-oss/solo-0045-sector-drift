package domain

import (
	"fmt"
	"sort"
	"strings"
)

// groupIndex owns the disjoint asset -> group mapping and the atomic units the
// compiler reasons about.
type groupIndex struct {
	ids     []string
	groups  map[string]Group
	members map[string]Set
	byAsset map[string]string
}

func indexGroups(src []Group, assets map[string]Asset) (*groupIndex, error) {
	gi := &groupIndex{
		groups:  make(map[string]Group),
		members: make(map[string]Set),
		byAsset: make(map[string]string),
	}
	seen := make(Set)
	claimed := make(Set)
	for _, g := range src {
		id := g.ID
		if id == "" || len(id) > 64 || seen[id] {
			return nil, fmt.Errorf("invalid or repeated resource group %q", id)
		}
		memberIDs := g.Assets
		if len(memberIDs) == 0 && len(g.Members) > 0 {
			memberIDs = g.Members
		}
		if len(memberIDs) == 0 || len(memberIDs) > 128 {
			return nil, fmt.Errorf("resource group %q must list between 1 and 128 members", id)
		}
		if err := knownAssets(memberIDs, assets); err != nil {
			return nil, fmt.Errorf("resource group %q: %w", id, err)
		}
		for _, a := range memberIDs {
			if claimed[a] {
				return nil, fmt.Errorf("asset %q belongs to more than one resource group", a)
			}
			claimed[a] = true
			gi.byAsset[a] = id
		}
		seen[id] = true
		gi.groups[id] = g
		gi.members[id] = toSet(memberIDs)
		gi.ids = append(gi.ids, id)
	}
	sort.Strings(gi.ids)
	return gi, nil
}

func toSet(ids []string) Set {
	out := make(Set, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// Unit is an atomic residency unit: either a declared group or a single
// ungrouped asset.
type Unit struct {
	Key     string
	GroupID string
	Members []string
}

func (gi *groupIndex) groupOf(assetID string) string {
	if gi == nil {
		return ""
	}
	return gi.byAsset[assetID]
}

// unitOf returns the atomic unit that owns assetID.
func (gi *groupIndex) unitOf(assetID string) Unit {
	if gi != nil {
		if gid, ok := gi.byAsset[assetID]; ok {
			return Unit{Key: "group:" + gid, GroupID: gid, Members: gi.members[gid].sorted()}
		}
	}
	return Unit{Key: assetID, Members: []string{assetID}}
}

// unitAssets returns the asset ids carried by an operation.
func opUnits(op Op) []string {
	if len(op.Assets) > 0 {
		return op.Assets
	}
	if op.Asset != "" {
		return []string{op.Asset}
	}
	return nil
}

// unitOrder topologically orders atomic units by their external dependencies.
// Dependencies internal to a declared group are collapsed, so mutual references
// inside one group are legal. Any cycle between units is a rejection: it is
// either an undeclared cycle or one that leaves a group and winds back.
func unitOrder(assets map[string]Asset, gi *groupIndex) ([]Unit, error) {
	keyUnit := make(map[string]Unit)
	for id := range assets {
		u := gi.unitOf(id)
		if _, ok := keyUnit[u.Key]; !ok {
			keyUnit[u.Key] = u
		}
	}
	keyDeps := make(map[string][]string, len(keyUnit))
	for key, u := range keyUnit {
		deps := make(Set)
		inUnit := toSet(u.Members)
		for _, m := range u.Members {
			for _, d := range assets[m].After {
				// Edges internal to a declared group are collapsed: mutual
				// references (and member self loops) inside the group are legal.
				if u.GroupID != "" && inUnit[d] {
					continue
				}
				du := gi.unitOf(d)
				// For an ungrouped singleton a self dependency keeps a real
				// self edge so the cycle walk rejects it.
				deps[du.Key] = true
			}
		}
		keyDeps[key] = deps.sorted()
	}

	marks := make(map[string]uint8)
	order := make([]Unit, 0, len(keyUnit))
	var visit func(string) error
	visit = func(key string) error {
		switch marks[key] {
		case 1:
			return cycleErrorFor(keyUnit[key])
		case 2:
			return nil
		}
		marks[key] = 1
		for _, d := range keyDeps[key] {
			if err := visit(d); err != nil {
				return err
			}
		}
		marks[key] = 2
		order = append(order, keyUnit[key])
		return nil
	}
	for _, key := range sortedUnitKeys(keyUnit) {
		if err := visit(key); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func sortedUnitKeys(m map[string]Unit) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cycleErrorFor(u Unit) error {
	if u.GroupID != "" {
		return &CycleError{
			Groups:  []string{u.GroupID},
			Message: fmt.Sprintf("dependency cycle leaves and re-enters resource group %q", u.GroupID),
		}
	}
	return &CycleError{Message: fmt.Sprintf("dependency cycle at %s", u.Key)}
}

// CycleError marks an illegal dependency cycle.
type CycleError struct {
	Cycle   []string `json:"Cycle,omitempty"`
	Groups  []string `json:"Groups,omitempty"`
	Message string   `json:"error"`
}

func (e *CycleError) Error() string { return e.Message }

// BlockError reports that mandatory residency cannot fit the budget and names
// the resource groups involved in the overflow.
type BlockError struct {
	// Group is the single most likely blocking group; Groups lists every
	// declared group participating in the over-budget mandatory set.
	Group    string   `json:"Group,omitempty"`
	Groups   []string `json:"Groups,omitempty"`
	Blocker  string   `json:"Blocker,omitempty"`
	Blockers []string `json:"Blockers,omitempty"`

	Needed int64 `json:"Needed"`
	Budget int64 `json:"Budget"`
	Bytes  int64 `json:"Bytes,omitempty"`

	Message string `json:"error"`
}

func (e *BlockError) Error() string { return e.Message }

// mandatoryBlock builds the budget failure, naming every involved group so the
// caller can tell which group blocked the handover.
func mandatoryBlock(resident Set, assets map[string]Asset, gi *groupIndex, used, budget int64) error {
	gids := make(Set)
	if gi != nil {
		for id := range resident {
			if gid := gi.byAsset[id]; gid != "" {
				gids[gid] = true
			}
		}
	}
	groupIDs := gids.sorted()
	be := &BlockError{Groups: groupIDs, Blockers: groupIDs, Needed: used, Budget: budget}
	if len(groupIDs) == 0 {
		be.Message = fmt.Sprintf("mandatory residency %d exceeds %d bytes", used, budget)
		return be
	}
	be.Group = primaryBlocker(groupIDs, gi, assets)
	be.Blocker = be.Group
	be.Bytes = groupBytes(be.Group, gi, assets)
	quoted := make([]string, len(groupIDs))
	for i, g := range groupIDs {
		quoted[i] = fmt.Sprintf("%q", g)
	}
	if len(groupIDs) == 1 {
		be.Message = fmt.Sprintf("required resource group %s blocks handover: mandatory residency %d exceeds budget %d", quoted[0], used, budget)
	} else {
		be.Message = fmt.Sprintf("required resource groups [%s] block handover: mandatory residency %d exceeds budget %d", strings.Join(quoted, ", "), used, budget)
	}
	return be
}

func groupBytes(gid string, gi *groupIndex, assets map[string]Asset) int64 {
	if gi == nil {
		return 0
	}
	var n int64
	for m := range gi.members[gid] {
		n += assets[m].Bytes
	}
	return n
}

// primaryBlocker chooses one group to expose as the blocker. With several
// candidates we take the heaviest group (the one most likely unable to fit),
// breaking ties by id.
func primaryBlocker(ids []string, gi *groupIndex, assets map[string]Asset) string {
	best := ids[0]
	var bestBytes int64 = -1
	for _, id := range ids {
		n := groupBytes(id, gi, assets)
		if n > bestBytes || (n == bestBytes && id < best) {
			bestBytes = n
			best = id
		}
	}
	return best
}

// GroupInfo is the plan's description of a declared group.
type GroupInfo struct {
	ID      string   `json:"ID"`
	Assets  []string `json:"Assets"`
	Members []string `json:"Members"`
	Bytes   int64    `json:"Bytes"`

	// Cost aliases for Bytes.
	TotalBytes int64 `json:"TotalBytes"`
	Cost       int64 `json:"Cost"`
}

func (gi *groupIndex) info(assets map[string]Asset) []GroupInfo {
	if gi == nil || len(gi.ids) == 0 {
		return nil
	}
	out := make([]GroupInfo, 0, len(gi.ids))
	for _, id := range gi.ids {
		members := gi.members[id].sorted()
		b := groupBytes(id, gi, assets)
		out = append(out, GroupInfo{ID: id, Assets: members, Members: members, Bytes: b, TotalBytes: b, Cost: b})
	}
	return out
}
