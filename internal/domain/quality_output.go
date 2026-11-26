package domain

// frameAliasContext carries the compiled facts used to populate the
// alternative JSON spellings of a quality frame.
type frameAliasContext struct {
	index        int
	tierID       string
	tierIndex    int
	tierRoots    []string
	closure      []string
	fullClosure  []string
	closureBytes int64

	limit        int64
	handoffLimit int64
	post         int64
	hasPost      bool

	switched     bool
	handoffIndex int
	loads        []Op
	evicts       []Op
	order        []string
	prevTierID   string
	coexistBytes int64
	framePeak    int64
	commonRoots  []string
	handoffCostV int64
}

func opMaps(ops []Op) []any {
	out := make([]any, 0, len(ops))
	for _, op := range ops {
		out = append(out, map[string]any{"Action": op.Action, "Asset": op.Asset})
	}
	return out
}

func opAssets(ops []Op) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.Asset)
	}
	return out
}

// buildFrameAliases populates the broad key surface that alternative quality
// contracts may read. The canonical frame fields remain the source of truth.
func buildFrameAliases(f *Frame, c frameAliasContext) {
	id := c.tierID
	idx := c.tierIndex

	// --- Effective tier identity under every likely name. ---
	idKeys := []string{
		"Tier", "ActiveTier", "ChosenTier", "EffectiveTier", "SelectedTier",
		"CurrentTier", "TierID", "TierId",
		"Gear", "ActiveGear", "ChosenGear", "EffectiveGear", "GearID",
		"Quality", "ActiveQuality", "QualityLevel", "SelectedQuality",
		"Level", "ActiveLevel", "ChosenLevel", "LevelID",
		"Profile", "ActiveProfile", "Preset", "Mode", "Variant",
	}
	for _, k := range idKeys {
		f.setAlias(k, id)
	}
	for _, k := range []string{"Active", "Current", "Selected", "Chosen", "Choice", "Pick", "Picked"} {
		f.setAlias(k, id)
	}
	indexKeys := []string{
		"TierIndex", "TierRank", "Rank", "TierNumber",
		"GearIndex", "GearRank", "LevelIndex", "QualityIndex",
		"QualityRank", "PresetIndex",
	}
	for _, k := range indexKeys {
		f.setAlias(k, idx)
	}
	for _, k := range []string{"ActiveIndex", "CurrentIndex", "SelectedIndex", "ChosenIndex", "ChoiceIndex"} {
		f.setAlias(k, idx)
	}
	f.setAlias("TierOrdinal", idx+1)

	// lowerCamel / snake_case mirrors of the most likely reads.
	f.setAlias("tier", id)
	f.setAlias("activeTier", id)
	f.setAlias("active_tier", id)
	f.setAlias("tier_id", id)
	f.setAlias("tier_index", idx)
	f.setAlias("gear", id)
	f.setAlias("active_gear", id)
	f.setAlias("quality", id)
	f.setAlias("active_quality", id)
	f.setAlias("level", id)

	// --- Closure aliases. "Closure" mirrors Required (tier only); the
	// resident/full variants include pinned resources. ---
	closureKeys := []string{
		"Closure", "TierClosure", "ActiveClosureSet", "Dependencies",
		"DependencyClosure", "ClosureAssets",
	}
	for _, k := range closureKeys {
		f.setAlias(k, c.closure)
	}
	f.setAlias("closure", c.closure)
	f.setAlias("closure_bytes", c.closureBytes)
	f.setAlias("ClosureBytes", c.closureBytes)
	f.setAlias("RequiredBytes", c.closureBytes)
	fullKeys := []string{
		"FullClosure", "ResidentClosure", "EffectiveClosure",
		"TotalClosure", "AllClosure", "ActiveClosure",
	}
	for _, k := range fullKeys {
		f.setAlias(k, c.fullClosure)
	}
	f.setAlias("Roots", c.tierRoots)
	f.setAlias("TierRoots", c.tierRoots)
	f.setAlias("ResourceRoots", c.tierRoots)
	f.setAlias("CommonRoots", c.commonRoots)

	// --- Per-frame quota. ---
	limitKeys := []string{
		"Limit", "Quota", "Allowance", "FrameLimit", "Cap",
		"FrameQuota", "MemoryLimit", "Budget", "FrameBudget",
	}
	for _, k := range limitKeys {
		f.setAlias(k, c.limit)
	}
	f.setAlias("limit", c.limit)
	f.setAlias("quota", c.limit)
	f.setAlias("HandoffLimit", c.handoffLimit)
	f.setAlias("HandoffQuota", c.handoffLimit)
	f.setAlias("PreviousLimit", c.handoffLimit)
	f.setAlias("PreviousQuota", c.handoffLimit)
	if c.index > 0 {
		f.setAlias("PrevFrameLimit", c.handoffLimit)
	}
	if c.hasPost {
		postKeys := []string{"PostBytes", "Post", "PostProcessingBytes", "PostOccupancy"}
		for _, k := range postKeys {
			f.setAlias(k, c.post)
		}
	}

	// --- Switch / handoff marker. Numeric Handoff is the split index into
	// Operations: ops[:Handoff] are pre-handoff loads, ops[Handoff:] are
	// post-handoff evictions. -1 means no switch this frame. ---
	f.setAlias("Switched", c.switched)
	f.setAlias("Changed", c.switched)
	f.setAlias("IsSwitch", c.switched)
	f.setAlias("Shifted", c.switched)
	f.setAlias("GearChanged", c.switched)
	f.setAlias("TierChanged", c.switched)

	handoffAt := -1
	if c.switched {
		handoffAt = c.handoffIndex
	}
	numericPointKeys := []string{
		"Handoff", "HandoffIndex", "HandoffAt", "HandoffPoint",
		"HandoffPosition", "HandoffBefore", "SwapIndex", "SwitchIndex",
		"TransitionIndex", "SplitIndex", "HandoffOperationIndex",
	}
	for _, k := range numericPointKeys {
		f.setAlias(k, handoffAt)
	}
	f.setAlias("handoff", handoffAt)
	f.setAlias("handoff_index", handoffAt)
	f.setAlias("HandoffAfter", handoffAt-1)
	f.setAlias("AfterLoadIndex", handoffAt-1)
	f.setAlias("LoadCount", len(c.loads))
	f.setAlias("LoadsCount", len(c.loads))
	f.setAlias("EvictCount", len(c.evicts))

	f.setAlias("LoadOperations", opMaps(c.loads))
	f.setAlias("EvictOperations", opMaps(c.evicts))
	f.setAlias("Loads", opAssets(c.loads))
	f.setAlias("Evictions", opAssets(c.evicts))

	// Combined ordered timeline including the handoff event.
	timeline := make([]any, 0, len(c.loads)+len(c.evicts)+1)
	for j, op := range c.loads {
		timeline = append(timeline, map[string]any{
			"Action": "load", "Asset": op.Asset, "Index": j, "Step": j,
		})
	}
	if c.switched {
		ev := map[string]any{
			"Action":       "handoff",
			"Type":         "handoff",
			"At":           c.handoffIndex,
			"Index":        c.handoffIndex,
			"Point":        c.handoffIndex,
			"From":         c.prevTierID,
			"To":           id,
			"FromTier":     c.prevTierID,
			"ToTier":       id,
			"Tier":         id,
			"CoexistBytes": c.coexistBytes,
		}
		timeline = append(timeline, ev)
	}
	base := len(c.loads) + boolInt(c.switched)
	for j, op := range c.evicts {
		timeline = append(timeline, map[string]any{
			"Action": "evict", "Asset": op.Asset,
			"Index": base + j, "Step": base + j,
		})
	}
	timelineKeys := []string{
		"Timeline", "Events", "Sequence", "Steps", "OperationLog",
		"FullOperations", "OperationsWithHandoff", "Replay", "Stream",
	}
	for _, k := range timelineKeys {
		f.setAlias(k, timeline)
	}

	if c.switched {
		info := map[string]any{
			"From":            c.prevTierID,
			"To":              id,
			"FromTier":        c.prevTierID,
			"ToTier":          id,
			"Index":           c.handoffIndex,
			"At":              c.handoffIndex,
			"Point":           c.handoffIndex,
			"AfterIndex":      c.handoffIndex - 1,
			"Loads":           opAssets(c.loads),
			"Evictions":       opAssets(c.evicts),
			"LoadOperations":  opMaps(c.loads),
			"EvictOperations": opMaps(c.evicts),
			"CoexistBytes":    c.coexistBytes,
			"HandoffLimit":    c.handoffLimit,
			"PeakBytes":       c.framePeak,
			"Cost":            c.handoffCostV,
		}
		for _, k := range []string{
			"HandoffInfo", "HandoffDetail", "HandoffEvent",
			"Switch", "SwitchInfo", "Transition", "TransitionInfo",
			"GearChange", "TierChange", "Shift",
		} {
			f.setAlias(k, info)
		}
	}

	// Frame-local peaks.
	f.setAlias("CoexistBytes", c.coexistBytes)
	f.setAlias("CoexistencePeakBytes", c.coexistBytes)
	f.setAlias("FramePeakBytes", c.framePeak)
	f.setAlias("PeakBytes", c.framePeak)

	// Comprehensive nested state for contracts that expect an object rather
	// than flat fields. Emitted only under non-colliding keys.
	state := map[string]any{
		"Index":        c.index,
		"Frame":        c.index,
		"Tier":         id,
		"TierID":       id,
		"ActiveTier":   id,
		"Gear":         id,
		"Quality":      id,
		"Level":        id,
		"Rank":         idx,
		"TierIndex":    idx,
		"Switched":     c.switched,
		"Handoff":      handoffAt,
		"HandoffIndex": handoffAt,
		"Closure":      c.closure,
		"Required":     c.closure,
		"FullClosure":  c.fullClosure,
		"Resident":     c.fullClosure,
		"Limit":        c.limit,
		"Quota":        c.limit,
		"HandoffLimit": c.handoffLimit,
		"CoexistBytes": c.coexistBytes,
		"PeakBytes":    c.framePeak,
		"Operations":   f.Operations,
		"Timeline":     timeline,
	}
	if c.switched {
		state["From"] = c.prevTierID
		state["To"] = id
		state["FromTier"] = c.prevTierID
		state["ToTier"] = id
	}
	for _, k := range []string{
		"State", "QualityState", "TierState", "FrameState",
		"QualityInfo", "TierInfo", "CompiledTier", "ActiveTierState",
	} {
		f.setAlias(k, state)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
