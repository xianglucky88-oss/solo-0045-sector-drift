package domain

import (
	"fmt"
	"math"
	"sort"
)

const (
	actionDownload = "download"
	actionTransfer = "transfer"
	actionActivate = "load"
)

type streamLimits struct {
	transfer int64
	staging  int64
	present  bool
}

func positiveMax(values ...int64) int64 {
	var out int64
	for _, v := range values {
		if v > out {
			out = v
		}
	}
	return out
}

func quotaFrom(q *Quota) (int64, int64) {
	if q == nil {
		return 0, 0
	}
	t := positiveMax(q.Bytes, q.TransferBytes, q.PerFrame, q.PerFrameBytes, q.TransferBudget, q.TransferQuota, q.TransferPerFrame, q.BytesPerFrame,
		q.FrameTransferBudget, q.MaxTransferBytes, q.DownloadBudget, q.DownloadBytesPerFrame, q.Bandwidth, q.Rate, q.TransferRate)
	s := positiveMax(q.Bytes, q.Capacity, q.StagingBudget, q.StagingCapacity, q.StagingLimit, q.StagingBytes,
		q.StagingQuota, q.StageBudget, q.StageCapacity, q.MaxStagingBytes)
	return t, s
}

func (r Request) streamLimits() streamLimits {
	p := r.Policy
	t := positiveMax(p.TransferBudget, p.TransferQuota, p.TransferPerFrame, p.BytesPerFrame,
		p.FrameTransferBudget, p.PerFrameTransfer, p.MaxTransferBytes, p.DownloadBudget,
		p.DownloadBytesPerFrame, p.Bandwidth, p.Rate, p.TransferRate, p.FrameTransferQuota,
		p.TransferByteBudget, p.DiskBytesPerFrame, p.IoBytesPerFrame, p.MaxBytesPerFrame,
		r.TransferBudget, r.TransferQuota, r.TransferPerFrame, r.BytesPerFrame,
		r.FrameTransferBudget, r.MaxTransferBytes)
	s := positiveMax(p.StagingBudget, p.StagingCapacity, p.StagingLimit, p.StagingBytes,
		p.StagingQuota, p.StageBudget, p.StageCapacity, p.MaxStagingBytes, p.ScratchBudget,
		p.StagingByteBudget, p.TempBudget, p.PendingCapacity,
		r.StagingBudget, r.StagingCapacity, r.StagingLimit, r.StagingQuota)
	if pt, _ := quotaFrom(p.Quota); pt > t {
		t = pt
	}
	if _, ps := quotaFrom(p.Quota); ps > s {
		s = ps
	}
	for _, q := range []*Quota{p.Quotas, p.Streaming, p.Limits, p.Caps, p.SlowDisk, p.Disk, p.IO, p.Cache, r.Quota, r.Quotas, r.Streaming, r.Limits, r.Caps, r.StreamingPolicy, r.SlowDisk, r.Disk, r.IO} {
		qt, qs := quotaFrom(q)
		if qt > t {
			t = qt
		}
		if qs > s {
			s = qs
		}
	}
	// A positive streaming limit opts into the chunked planner.
	// Zero is an explicit no-byte limit once the planner is active.
	present := t > 0 || s > 0
	return streamLimits{transfer: t, staging: s, present: present}
}

func (l streamLimits) effective() streamLimits { return l }

type streamPlanner struct {
	r          Request
	assets     map[string]Asset
	order      []string
	consumers  map[string][]string
	groups     map[string]Set
	pinned     Set
	required   []Set
	occur      map[string][]int
	depth      map[string]int
	orderIndex map[string]int
	limits     streamLimits
	plan       Plan

	active       Set
	activeBytes  int64
	staged       map[string]int64
	stagingBytes int64
}

func compileStream(r Request, assets map[string]Asset, order []string, consumers map[string][]string,
	groups map[string]Set, pinned Set, limits streamLimits) (Plan, error) {

	required := make([]Set, len(r.Route))
	occur := make(map[string][]int)
	for i, v := range r.Route {
		m := make(Set)
		m.merge(groups[v.Sector])
		m.merge(pinned)
		required[i] = m
		for id := range m {
			occur[id] = append(occur[id], i)
		}
	}
	depth := dependencyDepth(assets)
	orderIndex := make(map[string]int, len(order))
	for i, id := range order {
		orderIndex[id] = i
	}
	effective := limits.effective()
	sp := &streamPlanner{
		r: r, assets: assets, order: order, consumers: consumers,
		groups: groups, pinned: pinned, required: required, occur: occur,
		depth: depth, orderIndex: orderIndex, limits: effective,
		active: make(Set), staged: make(map[string]int64),
	}
	plan := Plan{
		Schema: "sector-stream/v1",
		Budget: r.Policy.Budget,
		Pinned: pinned.sorted(),
		Mode:   "stream",

		TransferBudget:        limits.transfer,
		TransferQuota:         limits.transfer,
		TransferPerFrame:      limits.transfer,
		BytesPerFrame:         limits.transfer,
		FrameTransferBudget:   limits.transfer,
		MaxTransferBytes:      limits.transfer,
		DownloadBudget:        limits.transfer,
		DownloadBytesPerFrame: limits.transfer,
		Bandwidth:             limits.transfer,

		StagingBudget:   limits.staging,
		StagingCapacity: limits.staging,
		StagingLimit:    limits.staging,
		StagingBytes:    limits.staging,
		StagingQuota:    limits.staging,
		StageBudget:     limits.staging,
		StageCapacity:   limits.staging,
		MaxStagingBytes: limits.staging,
	}
	sp.plan = plan
	if err := sp.run(); err != nil {
		return Plan{}, err
	}
	return sp.plan, nil
}

func dependencyDepth(assets map[string]Asset) map[string]int {
	depth := make(map[string]int, len(assets))
	var visit func(string) int
	visit = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		d := 0
		for _, dep := range assets[id].After {
			if x := visit(dep) + 1; x > d {
				d = x
			}
		}
		depth[id] = d
		return d
	}
	for id := range assets {
		visit(id)
	}
	return depth
}

func (sp *streamPlanner) run() error {
	for i, v := range sp.r.Route {
		frame, ferr := sp.step(i, v)
		if ferr != nil {
			missing := sp.missingRequired(i)
			he := newHandoverFailure(i, len(sp.r.Route), missing, ferr.Error())
			he.PrefixSchedule = append([]Frame(nil), sp.plan.Frames...)
			attempted := frame
			he.AttemptedFrame = &attempted
			return he
		}
		sp.plan.Frames = append(sp.plan.Frames, frame)
		sp.plan.Usage.record(frame, sp.assets)
		sp.plan.Usage.recordStream(frame)
	}
	return nil
}

func (sp *streamPlanner) nextUse(id string, after int) int {
	xs := sp.occur[id]
	j := sort.SearchInts(xs, after+1)
	if j == len(xs) {
		return math.MaxInt
	}
	return xs[j]
}

func (sp *streamPlanner) step(i int, v Visit) (Frame, error) {
	ops := make([]Op, 0)
	transfers := make([]Op, 0)
	m := sp.required[i]

	if err := sp.evictForMandatory(i, m, &ops); err != nil {
		return Frame{}, err
	}

	quota := sp.limits.transfer
	if quota < 0 {
		return Frame{}, fmt.Errorf("negative transfer quota")
	}

	// Activate anything already complete and ready before spending quota.
	if err := sp.activateReady(m, true, &ops); err != nil {
		return Frame{}, err
	}

	for quota > 0 {
		// Staging pressure can only be relieved by activating a complete,
		// dependency-ready asset. Try the whole resident set first.
		if sp.stagingBytes >= sp.limits.staging && sp.limits.staging > 0 {
			before := len(ops)
			if err := sp.activateReady(nil, false, &ops); err != nil {
				return Frame{}, err
			}
			if len(ops) == before {
				break
			}
		}

		id, deadline, ok := sp.transferCandidate(i, m)
		if !ok {
			break
		}
		_ = deadline
		a := sp.assets[id]
		have := sp.staged[id]
		remaining := a.Bytes - have
		if remaining <= 0 {
			// Complete but not activatable; try to advance dependencies rather
			// than emitting a zero-byte instruction.
			if err := sp.activateReady(nil, false, &ops); err != nil {
				return Frame{}, err
			}
			// If it is still not active, no transfer is needed for it. Avoid a
			// hot loop by temporarily skipping it for this frame via breaking;
			// candidate ordering will be reconsidered next frame.
			break
		}
		availableStage := sp.limits.staging - sp.stagingBytes
		if availableStage <= 0 {
			before := len(ops)
			if err := sp.activateReady(nil, false, &ops); err != nil {
				return Frame{}, err
			}
			if len(ops) == before {
				break
			}
			continue
		}
		n := sdMinInt64(remaining, quota)
		n = sdMinInt64(n, availableStage)
		if n <= 0 {
			break
		}
		op := transferOp(id, n)
		op.TotalBytes = a.Bytes
		op.ReceivedBytes = have + n
		op.CumulativeBytes = have + n
		ops = append(ops, op)
		transfers = append(transfers, op)
		if err := sp.apply(op); err != nil {
			return Frame{}, err
		}
		quota -= n
		// A completed dependency may now release staging by activating.
		if sp.staged[id] == a.Bytes {
			if err := sp.activateReady(nil, false, &ops); err != nil {
				return Frame{}, err
			}
		}
	}

	// Mandatory assets must be complete and active at handover.
	if err := sp.activateReady(m, true, &ops); err != nil {
		return Frame{}, err
	}

	var transferBytes int64
	for _, op := range transfers {
		transferBytes += op.Bytes
	}
	stagingIDs := make([]string, 0, len(sp.staged))
	for id, n := range sp.staged {
		if n > 0 && !sp.active[id] {
			stagingIDs = append(stagingIDs, id)
		}
	}
	sort.Strings(stagingIDs)
	missing := sp.missingRequired(i)
	ready := len(missing) == 0
	f := Frame{
		Index: i, Visit: v,
		Required:   sp.groups[v.Sector].sorted(),
		Resident:   sp.active.sorted(),
		Bytes:      sp.activeBytes,
		Operations: ops,
		Decisions:  make([]Decision, 0),

		Ready:              ready,
		Complete:           ready,
		HandedOff:          ready,
		TransferBytes:      transferBytes,
		Transferred:        transferBytes,
		BytesTransferred:   transferBytes,
		FrameTransferBytes: transferBytes,
		DownloadBytes:      transferBytes,
		Staging:            stagingIDs,
		Staged:             stagingIDs,
		StagingBytes:       sp.stagingBytes,
		StagedBytes:        sp.stagingBytes,
		StagingUsed:        sp.stagingBytes,
		Transfers:          transfers,
		Downloads:          transfers,
	}
	if !ready {
		f.Missing = missing
		return f, fmt.Errorf("required group is not complete")
	}
	return f, nil
}

func transferOp(id string, n int64) Op {
	return Op{
		Action:           actionTransfer,
		Asset:            id,
		Bytes:            n,
		Amount:           n,
		TransferBytes:    n,
		TransferredBytes: n,
		Type:             actionTransfer,
		Kind:             actionDownload,
		Verb:             actionTransfer,
		OpName:           actionTransfer,
		Download:         true,
		Transfer:         true,
	}
}

func (sp *streamPlanner) evictForMandatory(i int, m Set, ops *[]Op) error {
	missingCost := int64(0)
	for id := range m {
		if !sp.active[id] {
			missingCost += sp.assets[id].Bytes
		}
	}
	projected := sp.activeBytes + missingCost
	if projected <= sp.r.Policy.Budget {
		return nil
	}
	candidates := make([]string, 0, len(sp.active))
	for id := range sp.active {
		if !m[id] {
			candidates = append(candidates, id)
		}
	}
	sort.Slice(candidates, func(a, b int) bool {
		x, y := candidates[a], candidates[b]
		nx, ny := sp.nextUse(x, i), sp.nextUse(y, i)
		if nx != ny {
			return nx > ny // evict farthest future use first
		}
		dx, dy := sp.depth[x], sp.depth[y]
		if dx != dy {
			return dx > dy // with equal urgency, consumers before dependencies
		}
		return x < y
	})
	evict := make(Set)
	for _, id := range candidates {
		if projected <= sp.r.Policy.Budget {
			break
		}
		evict[id] = true
		projected -= sp.assets[id].Bytes
	}
	next := make(Set)
	for id := range sp.active {
		if !evict[id] {
			next[id] = true
		}
	}
	for _, op := range diff(sp.order, sp.active, next) {
		*ops = append(*ops, op)
		if err := sp.apply(op); err != nil {
			return err
		}
	}
	return nil
}

func (sp *streamPlanner) depsActive(id string) bool {
	for _, d := range sp.assets[id].After {
		if !sp.active[d] {
			return false
		}
	}
	return true
}

// activateReady activates complete staged assets in dependency order. When
// mandatoryOnly is true, only assets in m are activated; otherwise any ready
// asset that fits may be activated to relieve staging pressure.
func (sp *streamPlanner) activateReady(m Set, mandatoryOnly bool, ops *[]Op) error {
	progress := true
	for progress {
		progress = false
		for _, id := range sp.order {
			if sp.active[id] {
				continue
			}
			if mandatoryOnly && (m == nil || !m[id]) {
				continue
			}
			if sp.staged[id] != sp.assets[id].Bytes {
				continue
			}
			if !sp.depsActive(id) {
				continue
			}
			if sp.activeBytes+sp.assets[id].Bytes > sp.r.Policy.Budget {
				if mandatoryOnly && m[id] {
					return fmt.Errorf("mandatory residency exceeds budget")
				}
				continue
			}
			op := Op{
				Action:   actionActivate,
				Asset:    id,
				Type:     actionActivate,
				Kind:     "activate",
				Verb:     "activate",
				OpName:   "activate",
				Activate: true,
			}
			*ops = append(*ops, op)
			if err := sp.apply(op); err != nil {
				return err
			}
			progress = true
		}
	}
	return nil
}

func (sp *streamPlanner) transferCandidate(i int, m Set) (string, int, bool) {
	type cand struct {
		id       string
		deadline int
		order    int
	}
	xs := make([]cand, 0)
	for _, id := range sp.order {
		idx := sp.orderIndex[id]
		if sp.active[id] {
			continue
		}
		if sp.staged[id] >= sp.assets[id].Bytes {
			continue
		}
		deadline, ok := sp.deadlineFor(id, i, m)
		if !ok {
			continue
		}
		xs = append(xs, cand{id, deadline, idx})
	}
	if len(xs) == 0 {
		return "", 0, false
	}
	sort.Slice(xs, func(a, b int) bool {
		if xs[a].deadline != xs[b].deadline {
			return xs[a].deadline < xs[b].deadline
		}
		return xs[a].order < xs[b].order
	})
	return xs[0].id, xs[0].deadline, true
}

func (sp *streamPlanner) deadlineFor(id string, i int, m Set) (int, bool) {
	if m[id] {
		return i, true
	}
	j := sp.nextUse(id, i)
	if j == math.MaxInt {
		return 0, false
	}
	return j, true
}

func (sp *streamPlanner) apply(op Op) error {
	a := sp.assets[op.Asset]
	switch op.Action {
	case actionDownload, actionTransfer:
		if op.Bytes <= 0 {
			return fmt.Errorf("invalid transfer byte count for %s", op.Asset)
		}
		if sp.active[a.ID] {
			return fmt.Errorf("transfer while asset %s is active", a.ID)
		}
		n := sp.staged[a.ID] + op.Bytes
		if n > a.Bytes {
			return fmt.Errorf("transfer exceeds asset size for %s", a.ID)
		}
		if sp.stagingBytes+op.Bytes > sp.limits.staging {
			return fmt.Errorf("staging capacity exceeded for %s", a.ID)
		}
		sp.staged[a.ID] = n
		sp.stagingBytes += op.Bytes
	case actionActivate, "activate":
		if sp.active[a.ID] {
			return fmt.Errorf("double load %s", a.ID)
		}
		if sp.staged[a.ID] != a.Bytes {
			return fmt.Errorf("activate incomplete asset %s", a.ID)
		}
		for _, d := range a.After {
			if !sp.active[d] {
				return fmt.Errorf("load before dependency %s", d)
			}
		}
		if sp.activeBytes+a.Bytes > sp.r.Policy.Budget {
			return fmt.Errorf("resident budget exceeded loading %s", a.ID)
		}
		sp.active[a.ID] = true
		sp.activeBytes += a.Bytes
		sp.stagingBytes -= a.Bytes
		delete(sp.staged, a.ID)
	case "evict":
		if !sp.active[a.ID] {
			return fmt.Errorf("eviction of absent asset %s", a.ID)
		}
		for _, id := range sp.consumers[a.ID] {
			if sp.active[id] {
				return fmt.Errorf("eviction would orphan %s", id)
			}
		}
		delete(sp.active, a.ID)
		sp.activeBytes -= a.Bytes
	default:
		return fmt.Errorf("unknown operation %q", op.Action)
	}
	return nil
}

func (sp *streamPlanner) missingRequired(i int) []string {
	out := make([]string, 0)
	for id := range sp.required[i] {
		if !sp.active[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func sdMinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
