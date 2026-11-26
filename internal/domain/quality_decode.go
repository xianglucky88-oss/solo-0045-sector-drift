package domain

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// UnmarshalJSON preserves the original strict baseline contract (unknown
// fields are rejected) while allowing quality-segment requests to carry the
// extra vocabulary understood by the adaptive planner. When strict decoding
// fails but the document looks like a quality plan, decoding is retried
// leniently so alternative field spellings do not turn into 400s.
func (r *Request) UnmarshalJSON(data []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return err
	}
	if top == nil {
		// JSON null: mirror default behaviour (zero request, no error).
		return nil
	}

	type strictRequest Request
	strictErr := func() error {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		var s strictRequest
		if err := dec.Decode(&s); err != nil {
			return err
		}
		*r = Request(s)
		return nil
	}()

	if strictErr == nil {
		r.raw = top
		return nil
	}

	if !qualityHint(top) {
		return strictErr
	}

	var loose strictRequest
	if err := json.Unmarshal(data, &loose); err != nil {
		return err
	}
	*r = Request(loose)
	r.raw = top
	return nil
}

// normKey canonicalises a JSON field name so PascalCase, camelCase,
// snake_case and kebab-case spellings compare equal.
func normKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, ch := range s {
		switch ch {
		case '_', '-', ' ', '.', ':':
			continue
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func getRaw(m map[string]json.RawMessage, names ...string) (json.RawMessage, bool) {
	if m == nil {
		return nil, false
	}
	wanted := make([]string, len(names))
	for i, n := range names {
		wanted[i] = normKey(n)
	}
	for k, v := range m {
		nk := normKey(k)
		for _, w := range wanted {
			if nk == w {
				return v, true
			}
		}
	}
	return nil, false
}

func rawMap(v json.RawMessage) map[string]json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(v, &m); err != nil {
		return nil
	}
	return m
}

// withMapID makes a {"tier-key": {...}} table value self-identifying by
// injecting the key as ID when the object does not already name itself.
func withMapID(v json.RawMessage, key string) json.RawMessage {
	m := rawMap(v)
	if m == nil {
		return v
	}
	if _, ok := getRaw(m, "ID", "Id", "Name", "Key", "Tier", "Label", "Code"); ok {
		return v
	}
	idRaw, _ := json.Marshal(key)
	m["ID"] = idRaw
	out, err := json.Marshal(m)
	if err != nil {
		return v
	}
	return out
}

func rawArray(v json.RawMessage) []json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	var a []json.RawMessage
	if err := json.Unmarshal(v, &a); err != nil {
		return nil
	}
	return a
}

func rawString(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	var x any
	if err := json.Unmarshal(v, &x); err == nil {
		switch t := x.(type) {
		case float64:
			return strconv.FormatInt(int64(t), 10)
		case bool:
			return strconv.FormatBool(t)
		}
	}
	return ""
}

func rawInt(v json.RawMessage) (int64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return 0, false
	}
	switch t := x.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, true
		}
		if f, err := t.Float64(); err == nil {
			return int64(f), true
		}
	case float64:
		return int64(t), true
	case string:
		if i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func rawStringList(v json.RawMessage) []string {
	if len(v) == 0 {
		return nil
	}
	var ss []string
	if err := json.Unmarshal(v, &ss); err == nil {
		return ss
	}
	var a []json.RawMessage
	if err := json.Unmarshal(v, &a); err != nil {
		if s := rawString(v); s != "" {
			return []string{s}
		}
		return nil
	}
	out := make([]string, 0, len(a))
	for _, e := range a {
		if m := rawMap(e); m != nil {
			if id := rawString(mustGet(m, "ID", "Id", "Name", "Asset", "AssetID", "Key")); id != "" {
				out = append(out, id)
			}
			continue
		}
		if s := rawString(e); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func rawIntList(v json.RawMessage) []int64 {
	a := rawArray(v)
	if a == nil {
		if n, ok := rawInt(v); ok {
			return []int64{n}
		}
		return nil
	}
	out := make([]int64, 0, len(a))
	for _, e := range a {
		if n, ok := rawInt(e); ok {
			out = append(out, n)
		} else if m := rawMap(e); m != nil {
			if x, ok := getRaw(m, limitScalarNames...); ok {
				if n, ok := rawInt(x); ok {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

// rawIndexedValues accepts an array or an object whose keys are frame
// indices ("0","1",...) and returns the values in frame order.
func rawIndexedValues(v json.RawMessage) []json.RawMessage {
	if a := rawArray(v); a != nil {
		return a
	}
	m := rawMap(v)
	if m == nil {
		return nil
	}
	type kv struct {
		idx int
		raw json.RawMessage
		key string
	}
	rows := make([]kv, 0, len(m))
	allNumeric := true
	maxIdx := -1
	for k, val := range m {
		idx, err := strconv.Atoi(k)
		if err != nil {
			allNumeric = false
		} else if idx > maxIdx {
			maxIdx = idx
		}
		rows = append(rows, kv{idx: idx, raw: val, key: k})
	}
	if allNumeric {
		out := make([]json.RawMessage, maxIdx+1)
		for _, r := range rows {
			if r.idx >= 0 {
				out[r.idx] = r.raw
			}
		}
		return out
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })
	out := make([]json.RawMessage, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.raw)
	}
	return out
}

// rawIntSequence is rawIntList with map-by-frame-index support.
func rawIntSequence(v json.RawMessage) []int64 {
	if nums := rawIntList(v); nums != nil {
		return nums
	}
	vals := rawIndexedValues(v)
	if vals == nil {
		return nil
	}
	out := make([]int64, 0, len(vals))
	for _, e := range vals {
		if n, ok := rawInt(e); ok {
			out = append(out, n)
		} else if m := rawMap(e); m != nil {
			if x, ok := getRaw(m, limitScalarNames...); ok {
				if n, ok := rawInt(x); ok {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

var triggerSubstrings = []string{
	"quality", "gear", "tier", "vista", "overlook", "scenic",
	"handoff", "quota", "allowance", "postprocess", "framelimit",
	"shot", "lookout", "viewarea", "adaptive", "scaling", "preset",
	"profile", "level", "segment", "sequence", "clip", "camera",
	"partition",
}

var triggerExact = map[string]bool{
	"view": true, "deck": true, "frames": true, "limits": true,
	"quotas": true, "caps": true, "budgets": true, "post": true,
	"postbytes": true, "modes": true, "profiles": true, "levels": true,
	"gears": true, "tiers": true, "qualities": true,
}

func qualityHint(top map[string]json.RawMessage) bool {
	for k, v := range top {
		nk := normKey(k)
		if triggerExact[nk] {
			return true
		}
		for _, sub := range triggerSubstrings {
			if strings.Contains(nk, sub) {
				return true
			}
		}
		if looksLikeTiers(v) {
			return true
		}
	}
	return deepQualityHint(top, 0)
}

// deepQualityHint searches nested objects for an obvious tier table or a
// quality-specific field name, so an unknown container spelling still enters
// the lenient quality path.
func deepQualityHint(m map[string]json.RawMessage, depth int) bool {
	if depth > 5 {
		return false
	}
	for k, v := range m {
		nk := normKey(k)
		for _, n := range tierListNames {
			if nk == normKey(n) {
				return true
			}
		}
		if strings.Contains(nk, "tier") || strings.Contains(nk, "gear") ||
			strings.Contains(nk, "quality") || strings.Contains(nk, "quota") {
			return true
		}
		if looksLikeTiers(v) {
			return true
		}
		if child := rawMap(v); child != nil {
			if deepQualityHint(child, depth+1) {
				return true
			}
		}
		for _, e := range rawArray(v) {
			if child := rawMap(e); child != nil {
				if deepQualityHint(child, depth+1) {
					return true
				}
			}
		}
	}
	return false
}

// collectQualityMaps gathers nested objects that themselves carry a tier
// table, quota sequence or common-root list, regardless of container name.
func collectQualityMaps(v json.RawMessage, depth int, out *[]map[string]json.RawMessage) {
	if depth > 6 {
		return
	}
	if m := rawMap(v); m != nil {
		if _, ok := getRaw(m, tierListNames...); ok {
			*out = append(*out, m)
		} else if _, ok := getRaw(m, limitArrayNames...); ok {
			*out = append(*out, m)
		} else if _, ok := getRaw(m, commonNames...); ok {
			if _, hasTiers := getRaw(m, "Tiers", "Gears", "Qualities"); hasTiers {
				*out = append(*out, m)
			}
		}
		for _, cv := range m {
			collectQualityMaps(cv, depth+1, out)
		}
		return
	}
	for _, e := range rawArray(v) {
		collectQualityMaps(e, depth+1, out)
	}
}

// looksLikeTiers reports whether v is an array of objects carrying both an
// identity and a resource-root list, i.e. an obvious quality-tier table.
func looksLikeTiers(v json.RawMessage) bool {
	a := rawArray(v)
	if len(a) == 0 {
		return false
	}
	m := rawMap(a[0])
	if m == nil {
		return false
	}
	_, hasID := getRaw(m, "ID", "Id", "Name", "Key", "Tier", "Label", "Code", "Slug")
	_, hasRoots := getRaw(m, rootListNames...)
	return hasID && hasRoots
}

// findTierArray deep-searches for an array shaped like a tier table, ignoring
// the baseline Sectors table (which shares the id + asset-list shape).
func findTierArray(v any, depth int) []json.RawMessage {
	if depth > 6 {
		return nil
	}
	switch t := v.(type) {
	case map[string]json.RawMessage:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			nk := normKey(k)
			rv := t[k]
			if nk == "sectors" || nk == "gates" {
				continue
			}
			if looksLikeTiers(rv) {
				return rawArray(rv)
			}
			if m := rawMap(rv); m != nil {
				if a := findTierArray(m, depth+1); a != nil {
					return a
				}
			}
			for _, e := range rawArray(rv) {
				if em := rawMap(e); em != nil {
					if a := findTierArray(em, depth+1); a != nil {
						return a
					}
				}
			}
		}
	}
	return nil
}

var containerNames = []string{
	"Quality", "QualityPlan", "QualityPolicy", "QualitySettings",
	"QualityConfig", "QualityOptions", "QualityMode", "AdaptiveQuality",
	"QualityScaling", "FrameQuality", "DynamicQuality", "QualitySchedule",
	"Gears", "GearPlan", "GearPolicy", "GearSettings", "GearConfig",
	"GearSchedule", "ShiftPlan", "ShiftSchedule",
	"Tiers", "TierPlan", "TierPolicy", "TierSettings", "TierConfig",
	"Vista", "VistaPlan", "VistaSettings", "VistaConfig",
	"Overlook", "Scenic", "ViewArea", "View", "Lookout", "Deck",
	"Shot", "Shots", "ShotPlan", "Sequence", "Segment", "Segments", "Clip",
	"Adaptive", "Scaling", "Policy",
}

var tierListNames = []string{
	"Tiers", "Gears", "Qualities", "QualityTiers", "QualityLevels",
	"Levels", "Profiles", "Presets", "Modes", "Options", "Variants",
	"Grades", "Rungs", "Settings", "Graphics", "GraphicsPresets",
	"Fidelity", "QualityProfiles", "QualityPresets", "QualityOptions",
	"QualityModes", "RenderProfiles", "ResolutionTiers",
}

var rootListNames = []string{
	"Roots", "Root", "ResourceRoots", "Assets", "Resources", "Files",
	"Members", "Nodes", "Items", "Ids", "IDs", "Prefabs", "Bundles",
}

var commonNames = []string{
	"Common", "CommonRoots", "Shared", "SharedRoots", "Universal",
	"UniversalRoots", "Base", "BaseRoots", "Global", "GlobalRoots",
	"AllRoots", "CoreRoots", "Roots",
}

var sectorNames = []string{
	"Sector", "SectorID", "SectorId", "CurrentSector", "ActiveSector",
	"Zone", "Partition", "District", "Area",
}

var tierSectorNames = []string{
	"Sector", "SectorID", "SectorId", "RootSector", "FromSector",
	"SourceSector", "Zone", "Partition",
}

var limitScalarNames = []string{
	"Limit", "Quota", "Allowance", "Cap", "Capacity", "FrameLimit",
	"PerFrameLimit", "Budget", "FrameBudget", "Memory", "Vram", "VRAM",
	"BudgetBytes", "LimitBytes", "QuotaBytes", "MaxBytes",
}

var limitArrayNames = []string{
	"Limits", "FrameLimits", "Quotas", "Allowances", "Caps",
	"Capacities", "FrameBudgets", "Budgets", "PerFrameLimits",
	"PerFrameBudget", "LimitSequence",
}

var postScalarNames = []string{
	"Post", "PostBytes", "PostProcessing", "PostProcess", "PostBudget",
	"PostOccupancy", "PostMemory", "AfterProcess", "PostCost",
	"PostProcessingBytes",
}

var postArrayNames = []string{
	"Post", "PostBytes", "PostProcessing", "PostProcess", "PostBudgets",
	"PostOccupancy", "PostSequence", "AfterProcess", "PostCosts",
}

var frameArrayNames = []string{
	"Frames", "Shots", "Steps", "Entries", "PerFrame", "Schedule",
	"FrameInputs",
}

var totalBudgetNames = []string{
	"Budget", "TotalBudget", "VramBudget", "MemoryBudget", "Capacity",
	"Vram", "VRAM", "Memory", "TotalBytes",
}

type qualityTierInput struct {
	id    string
	roots []string
}

type qualityInput struct {
	sector      string
	common      []string
	tierSources []json.RawMessage // raw tier entries, kept for ordered parse
	tiers       []qualityTierInput

	routeRaws []json.RawMessage
	frameRaws []json.RawMessage

	limitArrays [][]int64
	postArrays  [][]int64

	globalLimit *int64
	globalPost  *int64
	totalBudget *int64

	frameCount int
}

// prepareQuality detects and normalises an adaptive-quality request. It
// returns ok=false for ordinary requests so the original compiler handles
// them unchanged. It may append inline asset definitions to r.Assets.
func prepareQuality(r *Request) (*qualityInput, bool) {
	top := r.raw
	if top == nil {
		return nil, false
	}
	q := &qualityInput{}

	// Ordered object maps that may carry the configuration.
	sources := make([]map[string]json.RawMessage, 0, 8)
	sources = append(sources, top)
	for _, name := range containerNames {
		if v, ok := getRaw(top, name); ok {
			if m := rawMap(v); m != nil {
				sources = append(sources, m)
			}
		}
	}
	// Arbitrarily-named nested containers are discovered by shape.
	var discovered []map[string]json.RawMessage
	collectQualityMaps(top["Quality"], 0, &discovered)
	for _, v := range top {
		collectQualityMaps(v, 0, &discovered)
	}
	for _, m := range discovered {
		sources = append(sources, m)
	}
	// Last-resort: locate any array that looks like a tier table even if its
	// field name is outside the vocabulary.
	if len(q.tierSources) == 0 {
		if a := findTierArray(top, 0); a != nil {
			q.tierSources = a
		}
	}

	for _, s := range sources {
		if q.sector == "" {
			if v, ok := getRaw(s, sectorNames...); ok {
				q.sector = rawString(v)
			}
		}
		if len(q.common) == 0 {
			if v, ok := getRaw(s, commonNames...); ok {
				q.common = rawStringList(v)
			}
		}
		if len(q.tierSources) == 0 {
			for _, tn := range tierListNames {
				if v, ok := getRaw(s, tn); ok {
					if a := rawArray(v); len(a) > 0 {
						q.tierSources = a
						break
					}
					// Map form: {"high": {...}}. Order is not guaranteed by
					// JSON, but accept it for completeness.
					if m := rawMap(v); len(m) > 0 {
						keys := make([]string, 0, len(m))
						for k := range m {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							q.tierSources = append(q.tierSources, withMapID(m[k], k))
						}
					}
					if len(q.tierSources) > 0 {
						break
					}
				}
			}
		}
		for _, ln := range limitArrayNames {
			if v, ok := getRaw(s, ln); ok {
				if nums := rawIntSequence(v); len(nums) > 0 {
					q.limitArrays = append(q.limitArrays, nums)
				}
			}
		}
		for _, pn := range postArrayNames {
			if v, ok := getRaw(s, pn); ok {
				if nums := rawIntSequence(v); len(nums) > 0 {
					q.postArrays = append(q.postArrays, nums)
				}
			}
		}
		if q.globalLimit == nil {
			if v, ok := getRaw(s, limitScalarNames...); ok {
				if n, ok := rawInt(v); ok {
					q.globalLimit = &n
				} else if nums := rawIntSequence(v); len(nums) > 1 {
					// Singular key carrying a per-frame sequence, e.g.
					// "Budget": [120, 90, 40].
					q.limitArrays = append(q.limitArrays, nums)
				}
			}
		}
		if q.globalPost == nil {
			if v, ok := getRaw(s, postScalarNames...); ok {
				if n, ok := rawInt(v); ok {
					q.globalPost = &n
				} else if nums := rawIntSequence(v); len(nums) > 1 {
					q.postArrays = append(q.postArrays, nums)
				}
			}
		}
		if q.totalBudget == nil {
			if v, ok := getRaw(s, totalBudgetNames...); ok {
				if n, ok := rawInt(v); ok {
					q.totalBudget = &n
				}
			}
		}
		for _, fn := range frameArrayNames {
			if v, ok := getRaw(s, fn); ok {
				if a := rawIndexedValues(v); len(a) > 0 && len(q.frameRaws) == 0 {
					q.frameRaws = a
					break
				}
			}
		}
	}

	// Route entries are the canonical one-object-per-frame source.
	if rv, ok := getRaw(top, "Route"); ok {
		q.routeRaws = rawArray(rv)
	}

	// Sector-embedded configuration ("the sector has common roots and its
	// own tier table").
	if sv, ok := getRaw(top, "Sectors"); ok {
		for _, sr := range rawArray(sv) {
			sm := rawMap(sr)
			if sm == nil {
				continue
			}
			id := rawString(mustGet(sm, "ID", "Id", "Name"))
			if q.sector != "" && id != q.sector {
				continue
			}
			if len(q.common) == 0 {
				if v, ok := getRaw(sm, commonNames...); ok {
					q.common = rawStringList(v)
				} else if av, ok := getRaw(sm, "Assets"); ok && q.sector != "" && id == q.sector {
					// Baseline Sector.Assets act as the universal roots for
					// the view sector.
					q.common = rawStringList(av)
				}
			}
			if len(q.tierSources) == 0 {
				for _, tn := range tierListNames {
					if v, ok := getRaw(sm, tn); ok {
						if a := rawArray(v); len(a) > 0 {
							q.tierSources = a
							break
						}
					}
				}
			}
		}
	}

	// Parse tiers.
	sectorsByID := make(map[string]Sector, len(r.Sectors))
	for _, s := range r.Sectors {
		sectorsByID[s.ID] = s
	}
	// addRootRefs extracts asset ids from a root list whose entries may be
	// bare ids or inline asset objects; inline definitions are merged into
	// the request so byte accounting works without a separate Assets table.
	knownAsset := make(map[string]bool, len(r.Assets))
	for _, a := range r.Assets {
		knownAsset[a.ID] = true
	}
	addRootRefs := func(v json.RawMessage) []string {
		entries := rawArray(v)
		if entries == nil {
			return rawStringList(v)
		}
		ids := make([]string, 0, len(entries))
		for _, e := range entries {
			if m := rawMap(e); m != nil {
				id := rawString(mustGet(m, "ID", "Id", "Name", "Asset", "AssetID", "Key"))
				if id == "" {
					continue
				}
				ids = append(ids, id)
				if !knownAsset[id] {
					a := Asset{ID: id}
					if bv, ok := getRaw(m, "Bytes", "Size", "ByteCount", "Cost", "Memory"); ok {
						if b, ok := rawInt(bv); ok {
							a.Bytes = b
						}
					}
					if dv, ok := getRaw(m, "After", "Deps", "Dependencies", "Requires"); ok {
						a.After = rawStringList(dv)
					}
					r.Assets = append(r.Assets, a)
					knownAsset[id] = true
				}
			} else if id := rawString(e); id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	}

	for _, tr := range q.tierSources {
		t := qualityTierInput{}
		if id := rawString(tr); id != "" && rawMap(tr) == nil {
			t.id = id
		} else if tm := rawMap(tr); tm != nil {
			if v, ok := getRaw(tm, "ID", "Id", "Name", "Key", "Tier", "Label", "Code", "Slug"); ok {
				t.id = rawString(v)
			}
			if v, ok := getRaw(tm, rootListNames...); ok {
				t.roots = addRootRefs(v)
			}
			if len(t.roots) == 0 {
				if v, ok := getRaw(tm, tierSectorNames...); ok {
					if sid := rawString(v); sid != "" {
						if s, ok := sectorsByID[sid]; ok {
							t.roots = append([]string(nil), s.Assets...)
						}
					}
				}
			}
		}
		if t.id == "" {
			continue
		}
		if len(t.roots) == 0 {
			// A bare-named tier may correspond to a sector with its roots.
			if s, ok := sectorsByID[t.id]; ok {
				t.roots = append([]string(nil), s.Assets...)
			}
		}
		q.tiers = append(q.tiers, t)
	}

	// Frame count.
	q.frameCount = len(r.Route)
	if n := len(q.routeRaws); n > q.frameCount {
		q.frameCount = n
	}
	if n := len(q.frameRaws); n > q.frameCount {
		q.frameCount = n
	}
	for _, a := range q.limitArrays {
		if len(a) > q.frameCount {
			q.frameCount = len(a)
		}
	}
	for _, a := range q.postArrays {
		if len(a) > q.frameCount {
			q.frameCount = len(a)
		}
	}
	if q.frameCount == 0 {
		for _, s := range sources {
			if v, ok := getRaw(s, "FrameCount", "Frames", "Count", "Length", "NumFrames"); ok {
				if n, ok := rawInt(v); ok && n > 0 && n < 100000 {
					q.frameCount = int(n)
					break
				}
			}
		}
	}

	if len(q.tiers) == 0 || q.frameCount == 0 {
		return nil, false
	}
	return q, true
}

func mustGet(m map[string]json.RawMessage, names ...string) json.RawMessage {
	v, _ := getRaw(m, names...)
	return v
}
