package domain

import (
	"container/heap"
	"fmt"
	"sort"
)

// residency is a fixed-size asset bitset. The validated asset cap (512) fits
// exactly into eight uint64 words, and arrays are comparable map keys.
type residency [8]uint64

func bitIndex(i int) (word, bit uint) { return uint(i >> 6), uint(i & 63) }

func (b residency) has(i int) bool {
	w, bit := bitIndex(i)
	return b[w]&(uint64(1)<<bit) != 0
}
func (b *residency) set(i int) {
	w, bit := bitIndex(i)
	b[w] |= uint64(1) << bit
}
func (b residency) union(o residency) residency {
	for i := 0; i < 8; i++ {
		b[i] |= o[i]
	}
	return b
}
func (b residency) intersect(o residency) residency {
	for i := 0; i < 8; i++ {
		b[i] &= o[i]
	}
	return b
}
func (b residency) andNot(o residency) residency {
	for i := 0; i < 8; i++ {
		b[i] &^= o[i]
	}
	return b
}
func (b residency) subsetOf(o residency) bool {
	for i := 0; i < 8; i++ {
		if b[i]&^o[i] != 0 {
			return false
		}
	}
	return true
}
func (b residency) equal(o residency) bool { return b == o }

type legalMove struct {
	to       int
	gateID   string
	crossing bool
}

type searchEngine struct {
	assets     map[string]Asset
	order      []string
	assetIndex map[string]int
	indexAsset []string
	budget     int64

	signposts []Signpost
	spIndex   map[string]int
	gates     map[string][]Gate

	reqBits  []residency
	reqSet   []Set
	reqBytes []int64
	adj      [][]legalMove
}

func (e *searchEngine) bitsCost(b residency) int64 {
	var total int64
	for i, id := range e.indexAsset {
		if b.has(i) {
			total += e.assets[id].Bytes
		}
	}
	return total
}

func setToBits(s Set, idx map[string]int) residency {
	var b residency
	for id := range s {
		if i, ok := idx[id]; ok {
			b.set(i)
		}
	}
	return b
}

// buildEngine performs all shared indexing and constructs the legal move
// graph and per-signpost required closures.
func buildEngine(r Request, s Search) (*searchEngine, error) {
	if err := validateCommon(r); err != nil {
		return nil, err
	}
	if err := validateSearch(s); err != nil {
		return nil, err
	}
	assets, err := indexAssets(r.Assets)
	if err != nil {
		return nil, err
	}
	if err := knownAssets(r.Policy.Pinned, assets); err != nil {
		return nil, err
	}
	sectors, err := indexSectors(r.Sectors, assets)
	if err != nil {
		return nil, err
	}
	gates, err := connect(r.Gates, sectors)
	if err != nil {
		return nil, err
	}
	order, err := dependencyOrder(assets)
	if err != nil {
		return nil, err
	}

	indexAsset := append([]string(nil), order...)
	assetIndex := make(map[string]int, len(order))
	for i, id := range indexAsset {
		assetIndex[id] = i
	}

	groups := sectorClosures(sectors, assets)
	pinned := closure(r.Policy.Pinned, assets)

	e := &searchEngine{
		assets:     assets,
		order:      order,
		assetIndex: assetIndex,
		indexAsset: indexAsset,
		budget:     r.Policy.Budget,
		signposts:  s.Signposts,
		spIndex:    make(map[string]int, len(s.Signposts)),
		gates:      gates,
		reqBits:    make([]residency, len(s.Signposts)),
		reqSet:     make([]Set, len(s.Signposts)),
		reqBytes:   make([]int64, len(s.Signposts)),
		adj:        make([][]legalMove, len(s.Signposts)),
	}
	for i, sp := range s.Signposts {
		if sectors[sp.Sector].ID == "" {
			return nil, fmt.Errorf("signpost %q references unknown sector %q", sp.ID, sp.Sector)
		}
		if _, dup := e.spIndex[sp.ID]; dup {
			return nil, fmt.Errorf("duplicate signpost %q", sp.ID)
		}
		e.spIndex[sp.ID] = i
		req := make(Set)
		req.merge(groups[sp.Sector])
		req.merge(pinned)
		e.reqSet[i] = req
		e.reqBits[i] = setToBits(req, assetIndex)
		e.reqBytes[i] = e.bitsCost(e.reqBits[i])
	}

	// Canonical, de-duplicated adjacency.
	type edgeKey struct{ from, to, gate string }
	seenEdge := make(map[edgeKey]bool)
	for _, m := range s.Moves {
		fi, ok1 := e.spIndex[m.From]
		ti, ok2 := e.spIndex[m.To]
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("move references unknown signpost")
		}
		lm, ok := e.legalGate(fi, ti, m.Gate)
		if !ok {
			// An authored directed move that cannot clear a gate is simply
			// not traversable by the camera.
			continue
		}
		key := edgeKey{m.From, m.To, lm.gateID}
		if seenEdge[key] {
			continue
		}
		seenEdge[key] = true
		e.adj[fi] = append(e.adj[fi], lm)
	}
	for fi := range e.adj {
		sort.Slice(e.adj[fi], func(i, j int) bool {
			a, b := e.adj[fi][i], e.adj[fi][j]
			if a.to != b.to {
				return e.signposts[a.to].ID < e.signposts[b.to].ID
			}
			return a.gateID < b.gateID
		})
	}
	return e, nil
}

// legalGate applies the established crossing rule: same-sector moves need no
// gate; cross-sector moves need a directed gate reachable from the DEPARTURE
// signpost position.
func (e *searchEngine) legalGate(fi, ti int, named string) (legalMove, bool) {
	from := e.signposts[fi]
	to := e.signposts[ti]
	if from.Sector == to.Sector {
		return legalMove{to: ti}, true
	}
	gs := e.gates[from.Sector]
	for _, g := range gs { // connect already sorts by gate ID
		if g.To != to.Sector {
			continue
		}
		if named != "" && g.ID != named {
			continue
		}
		if abs(from.Position-g.At) <= g.Reach {
			return legalMove{to: ti, gateID: g.ID, crossing: true}, true
		}
	}
	return legalMove{}, false
}

// enumerateResidencies returns the inclusion-maximal feasible end-of-frame
// resident sets after fulfilling req from cur. Newly loaded assets are always
// within req; old residents may be retained when a dependency-closed set
// stays inside budget.
func (e *searchEngine) enumerateResidencies(cur, req residency) []residency {
	full := cur.union(req)
	if e.bitsCost(full) <= e.budget {
		return []residency{full}
	}
	reqCost := e.bitsCost(req)
	capOptional := e.budget - reqCost
	if capOptional < 0 {
		return nil
	}

	// Optional old assets in global dependency (topological) order.
	opts := make([]int, 0)
	for _, id := range e.order {
		i := e.assetIndex[id]
		if cur.has(i) && !req.has(i) {
			opts = append(opts, i)
		}
	}
	n := len(opts)
	optPos := make(map[int]int, n)
	for p, i := range opts {
		optPos[i] = p
	}
	// Direct dependencies that are also optional, as bitset masks.
	depMask := make([]residency, n)
	for p, i := range opts {
		a := e.assets[e.indexAsset[i]]
		var dm residency
		for _, d := range a.After {
			if di, ok := e.assetIndex[d]; ok && cur.has(di) && !req.has(di) {
				dm.set(optPos[di])
			}
		}
		depMask[p] = dm
	}
	byteAt := make([]int64, n)
	suffixBytes := make([]int64, n+1)
	for p, i := range opts {
		byteAt[p] = e.assets[e.indexAsset[i]].Bytes
	}
	for p := n - 1; p >= 0; p-- {
		suffixBytes[p] = suffixBytes[p+1] + byteAt[p]
	}

	// Baseline states always available: required only, plus a canonical
	// greedy-maximal retention. These also guarantee progress if the exact
	// subset search hits its safety node budget on a large instance.
	resultSet := make(map[residency]bool)
	addResult := func(b residency) { resultSet[b] = true }
	addResult(req)

	greedy := residency{}
	var greedyCost int64
	for p := 0; p < n; p++ {
		if depMask[p].subsetOf(greedy) && greedyCost+byteAt[p] <= capOptional {
			greedy.set(opts[p])
			greedyCost += byteAt[p]
		}
	}
	addResult(req.union(greedy))

	const maxEnumerationNodes = 300000
	nodes := 0
	var dfs func(pos int, chosen residency, chosenCost int64)
	dfs = func(pos int, chosen residency, chosenCost int64) {
		nodes++
		if nodes > maxEnumerationNodes {
			return
		}
		if pos == n {
			// Maximality: no unchosen optional asset can now be added.
			maximal := true
			for p, i := range opts {
				if chosen.has(i) {
					continue
				}
				if chosenCost+byteAt[p] > capOptional {
					continue // does not fit: not addable
				}
				if !depMask[p].subsetOf(chosen) {
					continue // missing retained dependencies: not addable
				}
				maximal = false // addable -> current set is not maximal
				break
			}
			if maximal {
				addResult(req.union(chosen))
			}
			return
		}
		// Completion shortcut: if every remaining asset fits byte-wise, the
		// unique maximal completion under this prefix includes all whose
		// dependencies remain satisfiable.
		if chosenCost+suffixBytes[pos] <= capOptional {
			completed := chosen
			cost := chosenCost
			for p := pos; p < n; p++ {
				i := opts[p]
				if completed.has(i) {
					continue
				}
				if depMask[p].subsetOf(completed) {
					completed.set(i)
					cost += byteAt[p]
				}
			}
			addResult(req.union(completed))
			return
		}

		i := opts[pos]
		// Include i only when all its optional dependencies were retained and
		// the byte budget still holds.
		if depMask[pos].subsetOf(chosen) && chosenCost+byteAt[pos] <= capOptional {
			next := chosen
			next.set(i)
			dfs(pos+1, next, chosenCost+byteAt[pos])
		}
		// Exclude i.
		dfs(pos+1, chosen, chosenCost)
	}
	dfs(0, residency{}, 0)

	out := make([]residency, 0, len(resultSet))
	for b := range resultSet {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return bitsLexLess(out[i], out[j]) })
	return out
}

func bitsLexLess(a, b residency) bool {
	for w := 0; w < 8; w++ {
		if a[w] != b[w] {
			return a[w] < b[w]
		}
	}
	return false
}

type stateKey struct {
	sp int
	b  residency
}

type sLabel struct {
	key     stateKey
	cost    int64
	moves   int
	pathKey string
	parent  *sLabel
	via     *legalMove
	index   int
}

func labelLess(a, b *sLabel) bool {
	if a.cost != b.cost {
		return a.cost < b.cost
	}
	if a.moves != b.moves {
		return a.moves < b.moves
	}
	if a.pathKey != b.pathKey {
		return a.pathKey < b.pathKey
	}
	return bitsLexLess(a.key.b, b.key.b)
}

type labelHeap []*sLabel

func (h labelHeap) Len() int { return len(h) }
func (h labelHeap) Less(i, j int) bool {
	return labelLess(h[i], h[j])
}
func (h labelHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *labelHeap) Push(x any) {
	l := x.(*sLabel)
	l.index = len(*h)
	*h = append(*h, l)
}
func (h *labelHeap) Pop() any {
	old := *h
	n := len(old)
	l := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return l
}

// labelDominates reports whether a is at least as good as b on both gross
// bytes and move count. For equal bytes and moves the lexicographically
// smaller path wins. Both dimensions must be retained on a Pareto front
// because a cheaper label using more moves can be infeasible under the cap
// while a costlier, shorter label still delivers the shot.
func labelDominates(a, b *sLabel) bool {
	if a.cost > b.cost || a.moves > b.moves {
		return false
	}
	if a.cost == b.cost && a.moves == b.moves {
		return a.pathKey <= b.pathKey
	}
	return true
}

// searchRoute returns the canonical optimal terminal label. Optimization
// order: minimum gross loaded bytes, then fewer moves, then lexicographically
// smallest signpost path (and residency) for deterministic output.
func (e *searchEngine) searchRoute(start, end, maxMoves int) (*sLabel, error) {
	if e.reqBytes[start] > e.budget {
		return nil, fmt.Errorf("start residency %d exceeds %d bytes", e.reqBytes[start], e.budget)
	}
	fronts := make(map[stateKey][]*sLabel)
	h := &labelHeap{}
	heap.Init(h)

	root := &sLabel{
		key:     stateKey{sp: start, b: e.reqBits[start]},
		cost:    e.reqBytes[start],
		moves:   0,
		pathKey: e.signposts[start].ID,
	}
	fronts[root.key] = []*sLabel{root}
	heap.Push(h, root)

	alive := func(cur *sLabel) bool {
		for _, l := range fronts[cur.key] {
			if l == cur {
				return true
			}
		}
		return false
	}
	consider := func(cand *sLabel) {
		fr := fronts[cand.key]
		for _, l := range fr {
			if labelDominates(l, cand) {
				return
			}
		}
		nf := make([]*sLabel, 0, len(fr)+1)
		for _, l := range fr {
			if labelDominates(cand, l) {
				continue
			}
			nf = append(nf, l)
		}
		nf = append(nf, cand)
		fronts[cand.key] = nf
		heap.Push(h, cand)
	}

	for h.Len() > 0 {
		cur := heap.Pop(h).(*sLabel)
		if !alive(cur) {
			continue // superseded Pareto label
		}
		if cur.key.sp == end {
			return cur, nil
		}
		if cur.moves >= maxMoves {
			continue
		}
		for _, mv := range e.adj[cur.key.sp] {
			reqB := e.reqBits[mv.to]
			if e.reqBytes[mv.to] > e.budget {
				continue
			}
			options := e.enumerateResidencies(cur.key.b, reqB)
			for _, nxt := range options {
				added := reqB.andNot(cur.key.b)
				addCost := e.bitsCost(added)
				cand := &sLabel{
					key:     stateKey{sp: mv.to, b: nxt},
					cost:    cur.cost + addCost,
					moves:   cur.moves + 1,
					pathKey: cur.pathKey + "/" + e.signposts[mv.to].ID,
					parent:  cur,
					via:     &mv,
				}
				consider(cand)
			}
		}
	}
	return nil, fmt.Errorf("no deliverable route within %d moves", maxMoves)
}

// chain returns labels from start to chosen end.
func chain(answer *sLabel) []*sLabel {
	var rev []*sLabel
	for l := answer; l != nil; l = l.parent {
		rev = append(rev, l)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// residencySet converts a bitset back to a domain Set.
func (e *searchEngine) residencySet(b residency) Set {
	out := make(Set)
	for i, id := range e.indexAsset {
		if b.has(i) {
			out[id] = true
		}
	}
	return out
}
