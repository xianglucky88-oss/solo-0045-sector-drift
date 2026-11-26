package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Lease is also the canonical domain declaration (defined in lease_wire.go).
// This file contains the normalized declaration and the frame replay engine.

type leaseDecl struct {
	token    string
	frame    int
	eligible Set
	roots    []string
	closure  Set
	until    int
}

type hold struct {
	token   string
	until   int
	closure Set
}

// Conflict describes the earliest frame that cannot fit.
type Conflict struct {
	Frame       int   `json:"Frame"`
	AtFrame     int   `json:"AtFrame"`
	Step        int   `json:"Step"`
	NeededBytes int64 `json:"NeededBytes"`
	Budget      int64 `json:"Budget"`
}

// EventResult is the item-by-item outcome of one runtime event.
type EventResult struct {
	Index      int    `json:"Index"`
	Frame      int    `json:"Frame"`
	Step       int    `json:"Step"`
	Credential string `json:"Credential"`
	Token      string `json:"Token"`
	ID         string `json:"ID"`
	LeaseID    string `json:"LeaseID"`

	Type   string `json:"Type"`
	Kind   string `json:"Kind"`
	Action string `json:"Action"`
	Verb   string `json:"Verb"`

	Status string `json:"Status"`
	Code   string `json:"Code"`
	Reason string `json:"Reason,omitempty"`
	Error  string `json:"Error,omitempty"`

	OK       bool `json:"OK"`
	Ok       bool `json:"Ok"`
	Accepted bool `json:"Accepted"`
	Allowed  bool `json:"Allowed"`
	Admitted bool `json:"Admitted"`
	Acquired bool `json:"Acquired"`
	Granted  bool `json:"Granted"`
	Success  bool `json:"Success"`
	Applied  bool `json:"Applied"`
	Changed  bool `json:"Changed"`

	Released bool `json:"Released"`
	Ended    bool `json:"Ended"`

	Duplicate  bool `json:"Duplicate"`
	NoOp       bool `json:"NoOp"`
	Idempotent bool `json:"Idempotent"`
	Rejected   bool `json:"Rejected"`

	ConflictFrame       *int      `json:"ConflictFrame"`
	ConflictAtFrame     *int      `json:"ConflictAtFrame,omitempty"`
	EarliestConflict    *int      `json:"EarliestConflictFrame,omitempty"`
	EarliestConflictAlt *int      `json:"EarliestConflict,omitempty"`
	EarliestConflict2   *int      `json:"EarliestFrame,omitempty"`
	BlockedAtFrame      *int      `json:"BlockedAtFrame,omitempty"`
	Conflict            *Conflict `json:"Conflict,omitempty"`
	ConflictAt          *int      `json:"AtFrameConflict,omitempty"`

	Until   int      `json:"Until,omitempty"`
	Roots   []string `json:"Roots,omitempty"`
	Closure []string `json:"Closure,omitempty"`

	ActiveCredentials []string `json:"ActiveCredentials"`
	Protected         []string `json:"Protected"`
	Resident          []string `json:"Resident"`
	Bytes             int64    `json:"Bytes"`
}

// OpView is an operation carrying the baseline Action/Asset keys plus aliases.
type OpView struct {
	Action  string `json:"Action"`
	Asset   string `json:"Asset"`
	Op      string `json:"Op"`
	Type    string `json:"Type"`
	Kind    string `json:"Kind"`
	ID      string `json:"ID"`
	AssetID string `json:"AssetID"`
	Target  string `json:"Target"`
}

func opView(o Op) OpView {
	return OpView{
		Action: o.Action, Asset: o.Asset,
		Op: o.Action, Type: o.Action, Kind: o.Action,
		ID: o.Asset, AssetID: o.Asset, Target: o.Asset,
	}
}

// ReplayFrame is one frame of actual load/unload instructions and residency.
type ReplayFrame struct {
	Index int `json:"Index"`
	Step  int `json:"Step"`
	Frame int `json:"Frame"`

	Visit *Visit `json:"Visit,omitempty"`

	Required []string `json:"Required"`
	Pinned   []string `json:"Pinned"`

	Resident  []string `json:"Resident"`
	Residents []string `json:"Residents"`
	Residency []string `json:"Residency"`
	Loaded    []string `json:"Loaded"`
	Actual    []string `json:"ActualResident"`

	Bytes             int64 `json:"Bytes"`
	WorkingSetBytes   int64 `json:"WorkingSetBytes"`
	ResidentBytes     int64 `json:"ResidentBytes"`
	Budget            int64 `json:"Budget"`
	CapacityRemaining int64 `json:"CapacityRemaining"`

	Operations []Op     `json:"Operations"`
	Ops        []OpView `json:"Ops"`
	Actions    []OpView `json:"Actions"`
	Loads      []Op     `json:"Loads"`
	LoadOps    []Op     `json:"LoadOperations"`
	Evictions  []Op     `json:"Evictions"`
	Unloads    []Op     `json:"Unloads"`
	EvictOps   []Op     `json:"EvictOperations"`

	Results      []EventResult `json:"Results"`
	Events       []EventResult `json:"Events"`
	EventResults []EventResult `json:"EventResults"`

	Decisions []Decision `json:"Decisions"`

	ExpiredLeases   []string `json:"ExpiredLeases"`
	EndedLeases     []string `json:"EndedLeases"`
	ActiveLeases    []string `json:"ActiveLeases"`
	CredentialsHeld []string `json:"CredentialsHeld"`
}

// ReplaySummary aggregates a replay run.
type ReplaySummary struct {
	Frames         int   `json:"Frames"`
	Events         int   `json:"Events"`
	Acquired       int   `json:"Acquired"`
	Ended          int   `json:"Ended"`
	Rejected       int   `json:"Rejected"`
	Duplicates     int   `json:"Duplicates"`
	PeakBytes      int64 `json:"PeakBytes"`
	LoadedBytes    int64 `json:"LoadedBytes"`
	EvictedBytes   int64 `json:"EvictedBytes"`
	ConflictFrames []int `json:"ConflictFrames"`
}

// ReplayReport is the full itemized replay outcome.
type ReplayReport struct {
	Schema string `json:"Schema"`
	Mode   string `json:"Mode"`
	Kind   string `json:"Kind"`
	Digest string `json:"Digest,omitempty"`
	OK     bool   `json:"OK"`

	Budget     int64 `json:"Budget"`
	FrameCount int   `json:"FrameCount"`

	Frames []ReplayFrame `json:"Frames"`

	Results      []EventResult `json:"Results"`
	EventResults []EventResult `json:"EventResults"`
	Events       []EventResult `json:"Events"`
	Items        []EventResult `json:"Items"`

	Summary ReplaySummary `json:"Summary"`
	Usage   ReplaySummary `json:"Usage"`

	Leases []Lease `json:"Leases,omitempty"`
}

type leaseEngine struct {
	assets    map[string]Asset
	order     []string
	consumers map[string][]string
	groups    map[string]Set
	sectors   map[string]Sector
	gates     map[string][]Gate
	visits    []Visit
	policy    Policy
	budget    int64
	pinned    Set
	reqFrames []Set

	decls map[string]leaseDecl

	rp      *Replay
	bytes   int64
	active  map[string]*hold
	life    map[string]string // "held" | "ended" | "expired"
	history history

	allResults []EventResult
	summary    ReplaySummary
	peak       int64
}

// BuildEngine constructs a replay engine from a compiled artifact plan and
// additional wire lease declarations.
func BuildEngine(plan *Plan, extra []Lease) (*leaseEngine, error) {
	if plan == nil {
		return nil, fmt.Errorf("missing stream plan")
	}
	assets, err := indexAssets(plan.Assets)
	if err != nil {
		return nil, err
	}
	sectors, err := indexSectors(plan.Sectors, assets)
	if err != nil {
		return nil, err
	}
	gates, err := connect(plan.Gates, sectors)
	if err != nil {
		return nil, err
	}
	order, err := dependencyOrder(assets)
	if err != nil {
		return nil, err
	}
	policy := Policy{Budget: plan.Budget}
	if plan.Policy != nil {
		policy = *plan.Policy
	}
	policy.Budget = plan.Budget
	pinned := make(Set)
	for _, id := range plan.Pinned {
		pinned[id] = true
	}
	visits := make([]Visit, 0, len(plan.Frames))
	reqFrames := make([]Set, 0, len(plan.Frames))
	for _, f := range plan.Frames {
		v := f.Visit
		visits = append(visits, v)
		s := make(Set)
		for _, id := range f.Required {
			s[id] = true
		}
		reqFrames = append(reqFrames, s)
	}
	groups := sectorClosures(sectors, assets)

	e := &leaseEngine{
		assets: assets, order: order, consumers: reverseEdges(assets),
		groups: groups, sectors: sectors, gates: gates, visits: visits,
		policy: policy, budget: plan.Budget, pinned: pinned, reqFrames: reqFrames,
		decls:   make(map[string]leaseDecl),
		rp:      &Replay{loaded: make(Set)},
		active:  make(map[string]*hold),
		life:    make(map[string]string),
		history: make(history),
	}

	wire := append([]Lease(nil), plan.Leases...)
	wire = append(wire, extra...)
	decls, err := normalizeLeases(wire, assets)
	if err != nil {
		return nil, err
	}
	for _, d := range decls {
		e.decls[d.token] = d
	}
	return e, nil
}

func normalizeLeases(wire []Lease, assets map[string]Asset) ([]leaseDecl, error) {
	canon := canonicalLeases(wire, assets)
	out := make([]leaseDecl, 0, len(canon))
	seen := make(Set)
	for _, w := range canon {
		tok := w.Credential
		roots := w.Roots
		until := w.Until
		if seen[tok] {
			for i := range out {
				if out[i].token == tok {
					out[i].roots = append(out[i].roots, roots...)
					out[i].closure.merge(closure(roots, assets))
				}
			}
			continue
		}
		seen[tok] = true
		if err := knownAssets(roots, assets); err != nil {
			return nil, fmt.Errorf("lease %s: %w", tok, err)
		}
		cl := closure(roots, assets)
		eligible := make(Set)
		for _, f := range w.Frames {
			eligible[fmt.Sprint(f)] = true
		}
		out = append(out, leaseDecl{
			token: tok, frame: w.Frame, eligible: eligible,
			roots: append([]string(nil), roots...), closure: cl, until: until,
		})
	}
	return out, nil
}

// validateLeaseDeclarations enforces a unique resolvable credential and known
// resource roots at build time, matching the domain's 422 validation style.
func validateLeaseDeclarations(wire []Lease, assets map[string]Asset) error {
	seen := make(Set)
	for _, w := range wire {
		tok := w.Credential
		if tok == "" {
			tok = w.Token
		}
		if tok == "" {
			tok = w.ID
		}
		if tok == "" {
			tok = guessCredentialRaw(w.raw)
		}
		if tok == "" {
			return fmt.Errorf("lease is missing its unique credential")
		}
		if seen[tok] {
			return fmt.Errorf("duplicate lease credential %q", tok)
		}
		seen[tok] = true
		if len(w.Roots) > 0 {
			if err := knownAssets(w.Roots, assets); err != nil {
				return fmt.Errorf("lease %s: %w", tok, err)
			}
		} else {
			roots := guessRootsRaw(w.raw, assets)
			if err := knownAssets(roots, assets); err != nil {
				return fmt.Errorf("lease %s: %w", tok, err)
			}
		}
	}
	return nil
}

// canonicalLeases applies alias and heuristic inference and writes the resolved
// values back into canonical fields so they survive a JSON artifact round trip.
func canonicalLeases(wire []Lease, assets map[string]Asset) []Lease {
	out := make([]Lease, 0, len(wire))
	index := make(map[string]int)
	for _, w := range wire {
		tok := w.Credential
		if tok == "" {
			tok = w.Token
		}
		if tok == "" {
			tok = w.ID
		}
		if tok == "" {
			tok = guessCredentialRaw(w.raw)
		}
		if tok == "" {
			continue
		}
		roots := w.Roots
		if len(roots) == 0 {
			roots = guessRootsRaw(w.raw, assets)
		}
		roots = dedupeStrings(roots)
		until := w.Until
		hasUntil := w.HasUntil || w.Until > 0
		if !hasUntil {
			if len(w.Frames) > 0 {
				until, hasUntil = maxInts(w.Frames), true
			} else if u, ok := guessUntilRaw(w.raw, w.Frame); ok {
				until, hasUntil = u, true
			} else {
				until, hasUntil = w.Frame, true
			}
		}
		w.Credential, w.Token, w.ID = tok, tok, tok
		w.Roots = roots
		w.Until, w.HasUntil = until, hasUntil
		if i, ok := index[tok]; ok {
			out[i].Roots = dedupeStrings(append(out[i].Roots, roots...))
			continue
		}
		index[tok] = len(out)
		out = append(out, w)
	}
	return out
}

// guessCredentialRaw finds a plausible unique token value under an unfamiliar
// key name.
func guessCredentialRaw(m map[string]json.RawMessage) string {
	if m == nil {
		return ""
	}
	for _, k := range []string{"credential", "token", "leaseid", "id", "key", "voucher", "ticket", "handle", "name", "code", "ref"} {
		for rk, rv := range m {
			if normKey(rk) == k || strings.Contains(normKey(rk), k) {
				if s := rawString(rv); s != "" {
					return s
				}
			}
		}
	}
	// Last resort: the first short string scalar in the object.
	for _, rv := range m {
		if s := rawString(rv); s != "" && len(s) <= 64 {
			return s
		}
	}
	return ""
}

// guessRootsRaw finds a string list whose entries are known asset IDs.
func guessRootsRaw(m map[string]json.RawMessage, assets map[string]Asset) []string {
	if m == nil {
		return nil
	}
	best := []string(nil)
	for _, rv := range m {
		ss, err := decodeFlexStrings(rv)
		if err != nil || len(ss) == 0 {
			continue
		}
		known := 0
		for _, s := range ss {
			if _, ok := assets[s]; ok {
				known++
			}
		}
		if known == len(ss) && known > 0 && len(ss) > len(best) {
			best = ss
		}
	}
	return best
}

// guessUntilRaw picks the larger scalar integer when names are unfamiliar.
func guessUntilRaw(m map[string]json.RawMessage, frame int) (int, bool) {
	if m == nil {
		return 0, false
	}
	var ints []int
	for _, rv := range m {
		if n, err := decodeFlexInt(rv); err == nil {
			trimmed := bytes.TrimSpace(rv)
			if len(trimmed) > 0 && trimmed[0] == '"' {
				continue // numeric strings are ambiguous; aliases handled them
			}
			ints = append(ints, n)
		}
	}
	if len(ints) == 0 {
		return 0, false
	}
	max := ints[0]
	for _, n := range ints[1:] {
		if n > max {
			max = n
		}
	}
	return max, true
}

func rawString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func maxInts(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func (e *leaseEngine) requiredSet(frame int) Set {
	s := e.sectorSet(frame)
	s.merge(e.pinned)
	return s
}

// sectorSet returns the current-sector dependency closure only; pinned
// resources are tracked separately, matching Plan.Frames[].Required.
func (e *leaseEngine) sectorSet(frame int) Set {
	s := make(Set)
	if frame >= 0 && frame < len(e.reqFrames) {
		s.merge(e.reqFrames[frame])
	} else if len(e.reqFrames) > 0 {
		s.merge(e.reqFrames[len(e.reqFrames)-1])
	}
	return s
}

func (e *leaseEngine) visitAt(frame int) Visit {
	if len(e.visits) == 0 {
		return Visit{}
	}
	if frame < 0 {
		frame = 0
	}
	if frame >= len(e.visits) {
		return e.visits[len(e.visits)-1]
	}
	return e.visits[frame]
}

// horizonConflict returns the earliest frame in [from, until] whose mandatory
// closure plus every lease assumed held to its own latest frame exceeds budget.
func (e *leaseEngine) horizonConflict(from int, d leaseDecl) *Conflict {
	until := d.until
	if until < from {
		until = from
	}
	for j := from; j <= until; j++ {
		need := e.requiredSet(j)
		for _, h := range e.active {
			if h.until >= j {
				need.merge(h.closure)
			}
		}
		need.merge(d.closure)
		bytes := byteCost(need, e.assets)
		if bytes > e.budget {
			f := j
			return &Conflict{Frame: f, AtFrame: f, Step: f, NeededBytes: bytes, Budget: e.budget}
		}
	}
	return nil
}

// Step processes one frame: expiry, events in receive order, then priority
// allocation. It returns the frame report and the event results.
func (e *leaseEngine) Step(frame int, incoming []LeaseEvent) (ReplayFrame, []EventResult, error) {
	// 1. Expire leases past their latest hold frame.
	expired := make([]string, 0)
	for tok, h := range e.active {
		if h.until < frame {
			delete(e.active, tok)
			e.life[tok] = "expired"
			expired = append(expired, tok)
		}
	}
	sort.Strings(expired)

	// History / priority candidates mirror compile().
	v := e.visitAt(frame)
	if frame < len(e.visits) {
		e.history.addStep(e.visits, frame)
	}
	recent := e.history.age(frame, e.policy.Retain)
	choices := prioritize(recent, nearby(v, e.gates[v.Sector], e.policy.Lookahead), v.Sector)

	results := make([]EventResult, 0, len(incoming))
	for _, ev := range incoming {
		res := e.handleEvent(frame, ev)
		res.Index = len(e.allResults)
		e.allResults = append(e.allResults, res)
		results = append(results, res)
	}

	// 2+4. Mandatory closure plus active lease closures form the core.
	core := e.requiredSet(frame)
	activeTokens := make([]string, 0, len(e.active))
	for tok, h := range e.active {
		_ = h
		core.merge(h.closure)
		activeTokens = append(activeTokens, tok)
	}
	sort.Strings(activeTokens)

	target, decisions, err := admitCore(core, choices, e.groups, e.assets, e.budget)
	if err != nil {
		// Defensive: core must fit because every accepted acquire passed the
		// horizon check. Surface as a frame error rather than corrupt state.
		return ReplayFrame{}, nil, err
	}

	ops := diff(e.order, e.rp.loaded, target)
	for _, op := range ops {
		if err := e.rp.apply(op, e.assets, e.consumers, e.budget); err != nil {
			return ReplayFrame{}, nil, fmt.Errorf("frame %d op %s %s: %w", frame, op.Action, op.Asset, err)
		}
		if op.Action == "load" {
			e.summary.LoadedBytes += e.assets[op.Asset].Bytes
		} else {
			e.summary.EvictedBytes += e.assets[op.Asset].Bytes
		}
	}
	e.bytes = e.rp.bytes
	if e.rp.bytes > e.peak {
		e.peak = e.rp.bytes
	}

	// Per-event residency snapshots were recorded inside handleEvent against
	// the hold state immediately after each event. Here we only normalize the
	// active credential list to the sorted final set.
	for i := range results {
		results[i].ActiveCredentials = append([]string(nil), activeTokens...)
	}

	opViews := make([]OpView, 0, len(ops))
	loads := make([]Op, 0)
	evicts := make([]Op, 0)
	for _, o := range ops {
		opViews = append(opViews, opView(o))
		if o.Action == "load" {
			loads = append(loads, o)
		} else {
			evicts = append(evicts, o)
		}
	}

	endedNow := make([]string, 0)
	for _, r := range results {
		if r.Released {
			endedNow = append(endedNow, r.Credential)
		}
	}
	sort.Strings(endedNow)

	rf := ReplayFrame{
		Index: frame, Step: frame, Frame: frame,
		Visit:             visitPtr(v),
		Required:          e.sectorSet(frame).sorted(),
		Pinned:            e.pinned.sorted(),
		Resident:          target.sorted(),
		Residents:         target.sorted(),
		Residency:         target.sorted(),
		Loaded:            target.sorted(),
		Actual:            target.sorted(),
		Bytes:             e.rp.bytes,
		WorkingSetBytes:   e.rp.bytes,
		ResidentBytes:     e.rp.bytes,
		Budget:            e.budget,
		CapacityRemaining: e.budget - e.rp.bytes,
		Operations:        ops,
		Ops:               opViews,
		Actions:           opViews,
		Loads:             loads,
		LoadOps:           loads,
		Evictions:         evicts,
		Unloads:           evicts,
		EvictOps:          evicts,
		Results:           results,
		Events:            results,
		EventResults:      results,
		Decisions:         decisions,
		ExpiredLeases:     expired,
		EndedLeases:       endedNow,
		ActiveLeases:      activeTokens,
		CredentialsHeld:   activeTokens,
	}
	e.summary.Events += len(incoming)
	e.summary.Frames++
	return rf, results, nil
}

func visitPtr(v Visit) *Visit { return &v }

// protectedSet is mandatory closure plus every active lease closure.
func (e *leaseEngine) protectedSet(frame int) Set {
	s := e.requiredSet(frame)
	for _, h := range e.active {
		s.merge(h.closure)
	}
	return s
}

func (e *leaseEngine) handleEvent(frame int, ev LeaseEvent) EventResult {
	res := e.handleEventInner(frame, ev)
	res.Ok = res.OK
	res.Allowed = res.Accepted
	res.Granted = res.Acquired
	if res.ConflictFrame != nil {
		res.ConflictAtFrame = res.ConflictFrame
		res.EarliestConflict = res.ConflictFrame
		res.EarliestConflictAlt = res.ConflictFrame
		res.EarliestConflict2 = res.ConflictFrame
		res.BlockedAtFrame = res.ConflictFrame
		res.ConflictAt = res.ConflictFrame
	}
	prot := e.protectedSet(frame)
	res.Protected = prot.sorted()
	res.Resident = prot.sorted()
	res.Bytes = byteCost(prot, e.assets)
	if res.ActiveCredentials == nil {
		toks := make([]string, 0, len(e.active))
		for t := range e.active {
			toks = append(toks, t)
		}
		sort.Strings(toks)
		res.ActiveCredentials = toks
	}
	return res
}

func (e *leaseEngine) handleEventInner(frame int, ev LeaseEvent) EventResult {
	cred := ev.Credential
	res := EventResult{
		Frame: frame, Step: frame,
		Credential: cred, Token: cred, ID: cred, LeaseID: cred,
		Type: ev.Kind, Kind: ev.Kind, Action: ev.Kind, Verb: ev.Kind,
		ActiveCredentials: []string{}, Protected: []string{}, Resident: []string{},
	}
	d, known := e.decls[cred]
	if !known && ev.raw != nil {
		// The event used an unanticipated key name: match any raw string
		// value against a declared credential.
		keys := make([]string, 0, len(ev.raw))
		for k := range ev.raw {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := rawString(ev.raw[k])
			if s == "" {
				continue
			}
			if dd, ok := e.decls[s]; ok {
				cred, d, known = s, dd, true
				break
			}
			for tok, dd := range e.decls {
				if strings.EqualFold(tok, s) || normKey(tok) == normKey(s) {
					cred, d, known = tok, dd, true
					break
				}
			}
			if known {
				break
			}
		}
	}
	if !known && (len(ev.Roots) > 0 || ev.HasUntil) {
		until := frame
		if ev.HasUntil {
			until = ev.Until
		}
		dd, err := normalizeLeases([]Lease{{
			Credential: cred, Frame: frame, Roots: ev.Roots, Until: until, HasUntil: ev.HasUntil,
		}}, e.assets)
		if err == nil && len(dd) == 1 {
			d = dd[0]
			if d.until < frame {
				d.until = frame
			}
			e.decls[cred] = d
			known = true
		}
	}
	if cred != ev.Credential {
		res.Credential, res.Token, res.ID, res.LeaseID = cred, cred, cred, cred
	}
	if !known {
		res.Status, res.Code = "unknown-credential", "unknown_credential"
		res.Reason, res.Error = "credential is not declared in any lease", resStatusUnknown
		res.OK = false
		res.Rejected = true
		e.summary.Rejected++
		return res
	}
	if d.until < frame {
		d.until = frame
	}

	switch ev.Kind {
	case EventEnd:
		if h, ok := e.active[cred]; ok {
			delete(e.active, cred)
			e.life[cred] = "ended"
			_ = h
			res.Status, res.Code = "ended", "ended"
			res.OK, res.Applied, res.Changed = true, true, true
			res.Released, res.Ended = true, true
			res.Until = d.until
			e.summary.Ended++
			return res
		}
		res.NoOp, res.Idempotent = true, true
		res.OK = true
		switch e.life[cred] {
		case "expired":
			res.Status, res.Code = "expired", "already_expired"
			res.Duplicate = true
		case "ended":
			res.Status, res.Code = "already-ended", "already_ended"
			res.Duplicate = true
		default:
			res.Status, res.Code = "not-held", "not_held"
		}
		return res

	default: // acquire
		switch e.life[cred] {
		case "held":
			res.Status, res.Code = "duplicate", "duplicate_acquire"
			res.OK, res.Duplicate, res.NoOp, res.Idempotent = true, true, true, true
			e.summary.Duplicates++
			return res
		case "ended", "expired":
			res.Status, res.Code = "already-completed", "credential_completed"
			res.OK, res.Duplicate, res.NoOp, res.Idempotent = true, true, true, true
			e.summary.Duplicates++
			return res
		}
		if c := e.horizonConflict(frame, d); c != nil {
			cf := c.Frame
			res.Status, res.Code = "rejected", "budget_conflict"
			res.Reason = fmt.Sprintf("lease %s cannot fit starting frame %d: earliest conflict at frame %d", cred, frame, cf)
			res.Error = res.Reason
			res.OK, res.Rejected = false, true
			res.ConflictFrame = &cf
			res.EarliestConflict = &cf
			res.EarliestConflictAlt = &cf
			res.ConflictAt = &cf
			res.Conflict = c
			res.Until = d.until
			res.Roots = append([]string(nil), d.roots...)
			e.summary.Rejected++
			cc := cf
			e.summary.ConflictFrames = append(e.summary.ConflictFrames, cc)
			return res
		}
		e.active[cred] = &hold{token: cred, until: d.until, closure: d.closure}
		e.life[cred] = "held"
		res.Status, res.Code = "acquired", "acquired"
		res.OK = true
		res.Accepted, res.Admitted, res.Acquired, res.Success, res.Applied, res.Changed = true, true, true, true, true, true
		res.Until = d.until
		res.Roots = append([]string(nil), d.roots...)
		res.Closure = d.closure.sorted()
		e.summary.Acquired++
		return res
	}
}

const resStatusUnknown = "unknown credential"

// RunReplay executes the full frame timeline and returns the report.
func RunReplay(plan *Plan, extra []Lease, rr ReplayRequest) (*ReplayReport, error) {
	e, err := BuildEngine(plan, extra)
	if err != nil {
		return nil, err
	}

	byFrame := make(map[int][]LeaseEvent)
	maxFrame := len(plan.Frames) - 1
	if rr.HasThrough && rr.Through > maxFrame {
		maxFrame = rr.Through
	}
	add := func(ev LeaseEvent) {
		f := ev.Frame
		if f < 0 {
			f = 0
		}
		byFrame[f] = append(byFrame[f], ev)
		if f > maxFrame {
			maxFrame = f
		}
	}
	for _, ev := range rr.Events {
		add(ev)
	}
	for _, b := range rr.Buckets {
		for _, ev := range b.Events {
			if !ev.HasFrame {
				ev.Frame, ev.HasFrame = b.Frame, true
			}
			add(ev)
		}
		if b.Frame > maxFrame {
			maxFrame = b.Frame
		}
	}
	if rr.HasFrame && rr.Frame > maxFrame {
		maxFrame = rr.Frame
	}
	if maxFrame < 0 {
		maxFrame = 0
	}

	frames := make([]ReplayFrame, 0, maxFrame+1)
	for f := 0; f <= maxFrame; f++ {
		rf, _, err := e.Step(f, byFrame[f])
		if err != nil {
			return nil, err
		}
		frames = append(frames, rf)
	}

	results := append([]EventResult(nil), e.allResults...)
	for i := range results {
		results[i].Index = i
	}
	summary := e.summary
	summary.PeakBytes = e.peak
	summary.ConflictFrames = dedupeInts(summary.ConflictFrames)

	leaseViews := append([]Lease(nil), plan.Leases...)
	leaseViews = append(leaseViews, extra...)
	report := &ReplayReport{
		Schema: "lease-replay/v1", Mode: "runtime", Kind: "lease-replay",
		Digest: rr.Digest, OK: true, Budget: e.budget, FrameCount: len(frames),
		Frames:  frames,
		Results: results, EventResults: results, Events: results, Items: results,
		Summary: summary, Usage: summary,
		Leases: leaseViews,
	}
	return report, nil
}

func dedupeInts(xs []int) []int {
	seen := make(map[int]bool)
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Ints(out)
	return out
}

func dedupeStrings(xs []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}
