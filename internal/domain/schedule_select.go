package domain

type frameAsset struct {
	frame int
	id    string
	bytes int64
}

type change struct {
	frame int
	id    string
	bytes int64
}

type selector struct {
	opp       []opportunity
	loads     [][]frameAsset
	base      []Set
	baseBytes []int64
	budget    int64
	warm      []Set
	extra     []int64
	chosen    []bool
	best      []bool
	bestCount int
	nodes     int
	nodeLimit int
}

func buildLoads(o opportunity, base []Set, assets map[string]Asset) []frameAsset {
	var out []frameAsset
	for f := o.start; f <= o.arrival; f++ {
		for id := range o.demand {
			if base[f][id] {
				continue
			}
			out = append(out, frameAsset{frame: f, id: id, bytes: assets[id].Bytes})
		}
	}
	return out
}

func newSelector(opp []opportunity, base []Set, assets map[string]Asset, budget int64) *selector {
	n := len(base)
	s := &selector{
		opp:       opp,
		loads:     make([][]frameAsset, len(opp)),
		base:      base,
		baseBytes: make([]int64, n),
		budget:    budget,
		warm:      make([]Set, n),
		extra:     make([]int64, n),
		chosen:    make([]bool, len(opp)),
		nodeLimit: 5_000_000,
	}
	for f := range base {
		s.baseBytes[f] = byteCost(base[f], assets)
		s.warm[f] = make(Set)
	}
	for i, o := range opp {
		s.loads[i] = buildLoads(o, base, assets)
	}
	// A strong feasible lower bound materially improves branch-and-bound.
	rank := make([]int, len(opp))
	costOrder := make([]int, len(opp))
	for i := range opp {
		rank[i] = i
		costOrder[i] = i
	}
	s.best = s.greedy(rank)
	s.bestCount = countTrue(s.best)
	costOrder = insertionSortCost(costOrder, opp, assets)
	if candidate := s.greedy(costOrder); betterMask(candidate, s.best, opp) {
		s.best = candidate
		s.bestCount = countTrue(candidate)
	}
	return s
}

func insertionSortCost(in []int, opp []opportunity, assets map[string]Asset) []int {
	cost := make([]int64, len(opp))
	for i, o := range opp {
		cost[i] = byteCost(o.demand, assets)
	}
	out := append([]int(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && cost[out[j]] < cost[out[j-1]]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func countTrue(mask []bool) int {
	n := 0
	for _, v := range mask {
		if v {
			n++
		}
	}
	return n
}

func (s *selector) resetState() {
	for f := range s.warm {
		s.warm[f] = make(Set)
		s.extra[f] = 0
	}
	for i := range s.chosen {
		s.chosen[i] = false
	}
}

func (s *selector) tryAdd(i int) ([]change, bool) {
	var changes []change
	for _, a := range s.loads[i] {
		if s.warm[a.frame][a.id] {
			continue
		}
		if s.baseBytes[a.frame]+s.extra[a.frame]+a.bytes > s.budget {
			s.undo(changes)
			return nil, false
		}
		s.warm[a.frame][a.id] = true
		s.extra[a.frame] += a.bytes
		changes = append(changes, change{a.frame, a.id, a.bytes})
	}
	return changes, true
}

func (s *selector) undo(changes []change) {
	for _, c := range changes {
		if !s.warm[c.frame][c.id] {
			continue
		}
		delete(s.warm[c.frame], c.id)
		s.extra[c.frame] -= c.bytes
	}
}

func (s *selector) greedy(order []int) []bool {
	s.resetState()
	mask := make([]bool, len(s.opp))
	for _, i := range order {
		changes, ok := s.tryAdd(i)
		if !ok {
			continue
		}
		mask[i] = true
		// Greedy retains the allocation, so do not undo.  resetState at the
		// start of each greedy run clears frame maps.
		_ = changes
	}
	return mask
}

func betterMask(a, b []bool, opp []opportunity) bool {
	ca, cb := countTrue(a), countTrue(b)
	if ca != cb {
		return ca > cb
	}
	for i := range opp {
		ai, bi := false, false
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai == bi {
			continue
		}
		return ai
	}
	return false
}

func (s *selector) consider() {
	count := countTrue(s.chosen)
	if count < s.bestCount {
		return
	}
	candidate := append([]bool(nil), s.chosen...)
	if count > s.bestCount || betterMask(candidate, s.best, s.opp) {
		s.best = candidate
		s.bestCount = count
	}
}

func (s *selector) search() []bool {
	s.resetState()
	s.nodes = 0
	s.dfs(0, 0)
	return s.best
}

func (s *selector) dfs(i, count int) {
	s.nodes++
	if s.nodes > s.nodeLimit {
		// The requirement's instances are small exact-selection cases.  If a
		// pathological request reaches the guard, retaining the best feasible
		// lower bound is safer than making the HTTP request unbounded.
		return
	}
	s.consider()
	if i == len(s.opp) {
		return
	}
	if count+(len(s.opp)-i) < s.bestCount {
		return
	}

	// Lexicographic priority order: try inclusion first.
	if changes, ok := s.tryAdd(i); ok {
		s.chosen[i] = true
		s.dfs(i+1, count+1)
		s.chosen[i] = false
		s.undo(changes)
	}
	s.dfs(i+1, count)
}

func selectOpportunities(opp []opportunity, base []Set, assets map[string]Asset, budget int64) []bool {
	if len(opp) == 0 {
		return nil
	}
	s := newSelector(opp, base, assets, budget)
	return s.search()
}
