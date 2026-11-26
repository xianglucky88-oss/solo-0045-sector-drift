package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type frameWindow struct {
	start, end int
	exclusive  bool
}

type gateAvailability struct {
	open   []frameWindow
	closed []frameWindow
}

type rawCue struct {
	gate      string
	departure int
}

type showConfig struct {
	active bool
	lead   int
	window int
	cues   []rawCue
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			f, ferr := x.Float64()
			if ferr != nil {
				return 0, false
			}
			return int(f), true
		}
		return int(n), true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil {
				return 0, false
			}
			return int(f), true
		}
		return n, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func firstInt(values ...any) (int, bool) {
	for _, v := range values {
		if n, ok := asInt(v); ok {
			return n, true
		}
	}
	return 0, false
}

func firstString(values ...any) string {
	for _, v := range values {
		switch x := v.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(x) != "" {
				return x
			}
		default:
			b, err := json.Marshal(x)
			if err == nil {
				var s string
				if json.Unmarshal(b, &s) == nil && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "1" || s == "true" || s == "yes" || s == "on" || s == "enabled"
	default:
		if n, ok := asInt(x); ok {
			return n != 0
		}
		return false
	}
}

// parseWindows converts all supported interval spellings.  flatFramesOnly is
// used for keys whose wire meaning is an explicit list of frame numbers.
func parseWindows(v any, flatFramesOnly bool) ([]frameWindow, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case string:
		return parseWindowString(x)
	case map[string]any:
		if looksLikeWindowObject(x) {
			w, err := parseWindowObject(x)
			if err != nil {
				return nil, err
			}
			return []frameWindow{w}, nil
		}
		// Frame-set map: {"1": true, "2": "open"}.
		var out []frameWindow
		for key, val := range x {
			n, ok := asInt(key)
			if !ok {
				continue
			}
			if val == nil || truthy(val) {
				out = append(out, frameWindow{start: n, end: n})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
		return out, nil
	case FrameWindow:
		b, _ := json.Marshal(x)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		w, err := parseWindowObject(m)
		return []frameWindow{w}, err
	case []FrameWindow:
		out := make([]frameWindow, 0, len(x))
		for _, item := range x {
			b, _ := json.Marshal(item)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			w, err := parseWindowObject(m)
			if err != nil {
				return nil, err
			}
			out = append(out, w)
		}
		return out, nil
	case []any:
		if !flatFramesOnly && len(x) == 2 {
			a, ok1 := asInt(x[0])
			b, ok2 := asInt(x[1])
			if ok1 && ok2 {
				return []frameWindow{{start: a, end: b}}, nil
			}
		}
		var out []frameWindow
		for _, item := range x {
			// A flat array under OpenFrames is a list of explicit frames; an
			// array element which is itself a two-number pair is still a range.
			if flatFramesOnly {
				if arr, ok := item.([]any); ok && len(arr) == 2 {
					if a, ok1 := asInt(arr[0]); ok1 {
						if b, ok2 := asInt(arr[1]); ok2 {
							out = append(out, frameWindow{start: a, end: b})
							continue
						}
					}
				}
				if n, ok := asInt(item); ok {
					out = append(out, frameWindow{start: n, end: n})
					continue
				}
			}
			ws, err := parseWindows(item, false)
			if err != nil {
				return nil, err
			}
			out = append(out, ws...)
		}
		return out, nil
	default:
		if n, ok := asInt(v); ok {
			return []frameWindow{{start: n, end: n}}, nil
		}
		b, _ := json.Marshal(v)
		var generic any
		if err := json.Unmarshal(b, &generic); err != nil {
			return nil, fmt.Errorf("invalid frame window")
		}
		return parseWindows(generic, flatFramesOnly)
	}
}

func parseWindowString(s string) ([]frameWindow, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if strings.ContainsAny(s, ",;") {
		var out []frameWindow
		for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
			ws, err := parseWindowString(part)
			if err != nil {
				return nil, err
			}
			out = append(out, ws...)
		}
		return out, nil
	}
	for _, sep := range []string{"..", ":", "-", "—", "~"} {
		if strings.Contains(s, sep) {
			parts := strings.SplitN(s, sep, 2)
			a, ok1 := asInt(strings.TrimSpace(parts[0]))
			b, ok2 := asInt(strings.TrimSpace(parts[1]))
			if ok1 && ok2 {
				return []frameWindow{{start: a, end: b}}, nil
			}
		}
	}
	if n, ok := asInt(s); ok {
		return []frameWindow{{start: n, end: n}}, nil
	}
	return nil, fmt.Errorf("invalid frame window %q", s)
}

func mapInt(m map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if n, good := asInt(v); good {
				return n, true
			}
		}
	}
	return 0, false
}

func looksLikeWindowObject(m map[string]any) bool {
	windowKeys := map[string]bool{
		"start": true, "startframe": true, "begin": true, "first": true, "from": true, "fromframe": true, "low": true, "left": true,
		"end": true, "endframe": true, "finish": true, "last": true, "to": true, "toframe": true, "high": true, "right": true, "stop": true,
		"until": true, "before": true, "exclusiveend": true, "duration": true, "length": true, "framecount": true, "frames": true,
		"frame": true, "at": true, "index": true,
	}
	for k := range m {
		if windowKeys[normalizeKey(k)] {
			return true
		}
	}
	return false
}

func parseWindowObject(m map[string]any) (frameWindow, error) {
	start, hasStart := mapInt(m, "Start", "start", "StartFrame", "startFrame", "Begin", "begin", "First", "first", "From", "from", "FromFrame", "fromFrame", "Low", "low", "Left", "left")
	end, hasEnd := mapInt(m, "End", "end", "EndFrame", "endFrame", "Finish", "finish", "Last", "last", "To", "to", "ToFrame", "toFrame", "High", "high", "Right", "right", "Stop", "stop")
	w := frameWindow{start: start, end: end}
	if v, ok := m["Until"]; ok {
		if n, good := asInt(v); good {
			w.end, w.exclusive = n, true
			hasEnd = true
		}
	}
	if v, ok := m["Before"]; ok {
		if n, good := asInt(v); good {
			w.end, w.exclusive = n, true
			hasEnd = true
		}
	}
	if v, ok := m["ExclusiveEnd"]; ok {
		if n, good := asInt(v); good {
			w.end, w.exclusive = n, truthy(v)
			hasEnd = true
		}
	}
	if duration, ok := mapInt(m, "Duration", "duration", "Length", "length", "FrameCount", "frameCount", "Frames", "frames"); ok {
		if !hasEnd && hasStart {
			w.end = start + duration
			// Duration/length is a count: [start, start+duration).
			w.exclusive = true
			hasEnd = true
		}
	}
	if !hasStart && !hasEnd {
		// An object may itself be a single frame marker.
		if at, ok := mapInt(m, "Frame", "frame", "At", "at", "Index", "index"); ok {
			return frameWindow{start: at, end: at}, nil
		}
		return frameWindow{}, fmt.Errorf("frame window requires start and end")
	}
	if !hasStart {
		w.start = w.end
	}
	if !hasEnd {
		w.end = w.start
	}
	return w, nil
}

func gateAvailabilityFromGate(g *Gate) (gateAvailability, error) {
	var av gateAvailability
	type aliasPair struct {
		v      any
		frames bool
		closed bool
	}
	values := []aliasPair{
		{g.OpenFrames, true, false}, {g.Frames, true, false}, {g.OpenAt, true, false}, {g.OpenOn, true, false}, {g.AvailableFrames, true, false},
		{g.OpenWindows, false, false}, {g.OpenWindow, false, false}, {g.Windows, false, false}, {g.Window, false, false}, {g.Open, false, false},
		{g.OpenPeriods, false, false}, {g.OpenPeriod, false, false}, {g.Periods, false, false},
		{g.OpenIntervals, false, false}, {g.OpenInterval, false, false}, {g.Intervals, false, false},
		{g.OpenRanges, false, false}, {g.OpenRange, false, false}, {g.Ranges, false, false},
		{g.GateSchedule, false, false}, {g.Schedule, false, false}, {g.OpenSchedule, false, false}, {g.Availability, false, false},
		{g.OpenFrameWindows, false, false}, {g.FrameWindows, false, false}, {g.OpeningWindows, false, false}, {g.OpeningPeriods, false, false}, {g.OpeningIntervals, false, false},
		{g.ClosedFrames, true, true},
	}
	for _, item := range values {
		if item.v == nil {
			continue
		}
		ws, err := parseWindows(item.v, item.frames)
		if err != nil {
			return av, fmt.Errorf("gate %s: %w", g.ID, err)
		}
		if item.closed {
			av.closed = append(av.closed, ws...)
		} else {
			av.open = append(av.open, ws...)
		}
	}
	return av, nil
}

func frameInWindows(frame int, ws []frameWindow) bool {
	for _, w := range ws {
		if frame < w.start || frame > w.end {
			continue
		}
		if w.exclusive && frame == w.end {
			continue
		}
		return true
	}
	return false
}

func (a gateAvailability) openAt(frame int) bool {
	if frameInWindows(frame, a.closed) {
		return false
	}
	if len(a.open) == 0 {
		return true
	}
	return frameInWindows(frame, a.open)
}

func availabilityMap(r *Request) (map[string]gateAvailability, error) {
	out := make(map[string]gateAvailability)
	for i := range r.Gates {
		av, err := gateAvailabilityFromGate(&r.Gates[i])
		if err != nil {
			return nil, err
		}
		out[r.Gates[i].ID] = av
	}
	holders := []any{
		r.GateSchedules, r.GateWindows, r.OpenWindows, r.DoorSchedules,
		r.Policy.GateSchedules, r.Policy.GateWindows, r.Policy.OpenWindows, r.Policy.GateAvailability,
	}
	for key, value := range r.featureExtra {
		nk := normalizeKey(key)
		gateish := strings.Contains(nk, "gate") || strings.Contains(nk, "door")
		eventish := strings.Contains(nk, "cue") || strings.Contains(nk, "shot") || strings.Contains(nk, "opportunit") || strings.Contains(nk, "booking")
		if gateish && !eventish {
			holders = append(holders, value)
		}
	}
	// Top-level maps such as GateWindows: {"ramp": [[2,3]]}.
	for _, holder := range holders {
		if holder == nil {
			continue
		}
		// Top-level lists such as [{Gate:"ramp", OpenFrames:[1,2]}].
		if arr, ok := holder.([]any); ok {
			for _, item := range arr {
				m, err := anyStringMap(item)
				if err != nil {
					return nil, fmt.Errorf("invalid gate schedule list")
				}
				id := firstString(m["Gate"], m["GateID"], m["Door"], m["DoorID"], m["ID"])
				if id == "" {
					return nil, fmt.Errorf("gate schedule entry is missing a gate")
				}
				clone := make(map[string]any, len(m))
				for k, v := range m {
					clone[k] = v
				}
				clone["ID"] = id
				delete(clone, "Gate")
				delete(clone, "GateID")
				delete(clone, "Door")
				delete(clone, "DoorID")
				b, _ := json.Marshal(clone)
				var g Gate
				if err := json.Unmarshal(b, &g); err != nil {
					return nil, err
				}
				av, err := gateAvailabilityFromGate(&g)
				if err != nil {
					return nil, err
				}
				existing := out[id]
				existing.open = append(existing.open, av.open...)
				existing.closed = append(existing.closed, av.closed...)
				out[id] = existing
			}
			continue
		}
		m, ok := holder.(map[string]any)
		if !ok {
			b, err := json.Marshal(holder)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(b, &m); err != nil {
				// It may be a Schedule-shaped object with a Gates map.
				var s Schedule
				if json.Unmarshal(b, &s) == nil && s.Gates != nil {
					mm, merr := anyStringMap(s.Gates)
					if merr != nil {
						return nil, merr
					}
					m = mm
				} else {
					return nil, fmt.Errorf("invalid gate schedule map")
				}
			}
		}
		ids := make([]string, 0, len(m))
		for nestedKey, nested := range m {
			nk := normalizeKey(nestedKey)
			if nk == "gates" || nk == "gateschedules" || nk == "gatewindows" || nk == "openwindows" || nk == "doorschedules" {
				nm, err := anyStringMap(nested)
				if err == nil {
					for id, value := range nm {
						ws, perr := parseWindows(value, false)
						if perr != nil {
							return nil, fmt.Errorf("gate %s schedule: %w", id, perr)
						}
						av := out[id]
						av.open = append(av.open, ws...)
						out[id] = av
					}
				}
			}
		}
		for id := range m {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			ws, err := parseWindows(m[id], false)
			if err != nil {
				return nil, fmt.Errorf("gate %s schedule: %w", id, err)
			}
			av := out[id]
			av.open = append(av.open, ws...)
			out[id] = av
		}
	}
	return out, nil
}

func anyStringMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func cueFromAny(v any) (rawCue, bool) {
	m, err := anyStringMap(v)
	if err != nil {
		return rawCue{}, false
	}
	gate := firstString(m["Gate"], m["GateID"], m["Door"], m["DoorID"], m["ID"])
	frame, ok := firstInt(m["DepartureFrame"], m["DepartFrame"], m["Departure"], m["DepartAtFrame"], m["DepartAt"], m["FromFrame"], m["StartFrame"], m["AtFrame"], m["Frame"], m["Step"], m["Index"], m["At"])
	if gate == "" {
		for k, val := range m {
			nk := normalizeKey(k)
			if (strings.Contains(nk, "gate") || strings.Contains(nk, "door")) && !strings.Contains(nk, "window") {
				gate = firstString(val)
				if gate != "" {
					break
				}
			}
		}
	}
	if !ok {
		// Prefer departure-named frame keys, then any frame-ish key.  Warmup
		// start is not an input-cue field; such keys only appear in output.
		for _, preference := range []func(string) bool{
			func(k string) bool { return strings.Contains(k, "depart") },
			func(k string) bool {
				return strings.Contains(k, "fromframe") || k == "frame" || strings.Contains(k, "atframe") || k == "step" || k == "index" || k == "at"
			},
		} {
			for k, val := range m {
				nk := normalizeKey(k)
				if preference(nk) && !strings.Contains(nk, "warmup") && !strings.Contains(nk, "start") {
					if n, good := asInt(val); good {
						frame, ok = n, true
						break
					}
				}
			}
			if ok {
				break
			}
		}
	}
	if gate == "" || !ok {
		return rawCue{}, false
	}
	return rawCue{gate: gate, departure: frame}, true
}

func appendCues(dst *[]rawCue, v any) {
	if v == nil {
		return
	}
	switch x := v.(type) {
	case []rawCue:
		*dst = append(*dst, x...)
	case []Cue:
		for _, c := range x {
			if rc, ok := cueFromAny(c); ok {
				*dst = append(*dst, rc)
			}
		}
	case []any:
		for _, item := range x {
			if rc, ok := cueFromAny(item); ok {
				*dst = append(*dst, rc)
			}
		}
	default:
		if rc, ok := cueFromAny(v); ok {
			*dst = append(*dst, rc)
			return
		}
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		var cues []Cue
		if json.Unmarshal(b, &cues) == nil {
			for _, c := range cues {
				if rc, ok := cueFromAny(c); ok {
					*dst = append(*dst, rc)
				}
			}
			return
		}
		var arr []any
		if json.Unmarshal(b, &arr) == nil {
			for _, item := range arr {
				if rc, ok := cueFromAny(item); ok {
					*dst = append(*dst, rc)
				}
			}
		}
	}
}

func intFromSources(keys []string, sources ...any) (int, bool) {
	for _, src := range sources {
		m, err := anyStringMap(src)
		if err != nil {
			continue
		}
		for _, k := range keys {
			// Exact first, then case-insensitive lookup.
			if v, ok := m[k]; ok {
				if n, good := asInt(v); good {
					return n, true
				}
			}
		}
		for _, k := range keys {
			want := normalizeKey(k)
			for mk, mv := range m {
				if normalizeKey(mk) == want {
					if n, good := asInt(mv); good {
						return n, true
					}
				}
			}
		}
		if n, ok := fuzzyScheduleInt(m, keys); ok {
			return n, true
		}
	}
	return 0, false
}

func fuzzyScheduleInt(m map[string]any, keys []string) (int, bool) {
	lead := false
	window := false
	for _, k := range keys {
		nk := normalizeKey(k)
		if strings.Contains(nk, "warmup") || strings.Contains(nk, "prewarm") || strings.Contains(nk, "preheat") || strings.Contains(nk, "preroll") {
			lead = true
		}
		if nk == "lead" || strings.Contains(nk, "leadframe") || strings.Contains(nk, "advanceframe") || nk == "advance" {
			lead = true
		}
		if strings.Contains(nk, "window") || strings.Contains(nk, "horizon") || strings.Contains(nk, "lookaheadframe") || strings.Contains(nk, "framelookahead") {
			window = true
		}
	}
	for mk, mv := range m {
		nk := normalizeKey(mk)
		n, good := asInt(mv)
		if !good {
			continue
		}
		looksLead := strings.Contains(nk, "warm") || strings.Contains(nk, "prewarm") || strings.Contains(nk, "preheat") || strings.Contains(nk, "preroll") ||
			strings.Contains(nk, "preload") || strings.Contains(nk, "prepare") || strings.Contains(nk, "prime") ||
			strings.Contains(nk, "lead") || strings.Contains(nk, "ahead") || strings.Contains(nk, "advance")
		looksWindow := strings.Contains(nk, "window") || strings.Contains(nk, "horizon") || strings.Contains(nk, "span") ||
			strings.Contains(nk, "lookahead") || strings.Contains(nk, "frameview") || strings.Contains(nk, "searchframe")
		if lead && looksLead && !looksWindow {
			return n, true
		}
		if window && looksWindow {
			return n, true
		}
	}
	return 0, false
}

func normalizeKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func prepareShow(r Request) (showConfig, error) {
	cfg := showConfig{}
	extraConfig := map[string]any{}
	extraContainers := make([]any, 0)
	for key, value := range r.featureExtra {
		cfg.active = true
		switch value.(type) {
		case []any:
			appendCues(&cfg.cues, value)
		case map[string]any:
			extraContainers = append(extraContainers, value)
		default:
			extraConfig[key] = value
		}
	}
	containers := []any{
		r.Schedule, r.Show, r.ShowSchedule, r.Performance, r.Production, r.Staging, r.Cinematic, r.Sequence, r.Framing,
		r.ShowPlan, r.SchedulePlan, r.PerformancePlan, r.FramingPlan, r.CameraPlan, r.ShotList, r.ShotSchedule,
		r.WarmupPlan, r.WarmupScheduleInput, r.PreloadSchedule, r.PreheatSchedule, r.CueSheet, r.CallSheet, r.GateSchedule, r.DoorSchedule,
		r.Policy.Schedule, r.Policy.Show, r.Policy.ShowSchedule, r.Policy.Framing,
	}
	containers = append(containers, extraContainers...)
	for _, c := range containers {
		if c != nil {
			cfg.active = true
			break
		}
	}
	listHolders := []any{
		r.Cues, r.Opportunities, r.FramingOpportunities, r.Shots, r.PhotoOps, r.Vistas, r.Setups, r.Bookings,
		r.Takes, r.DoorShots, r.CameraCues, r.Calls, r.Policy.Cues, r.Policy.Shots,
	}
	for _, l := range listHolders {
		if l != nil {
			cfg.active = true
		}
	}

	leadKeys := []string{"WarmupFrames", "LeadFrames", "PreRollFrames", "PrerollFrames", "PrewarmFrames", "PreheatFrames", "AdvanceFrames", "WarmupLeadFrames", "Warmup", "WarmUp", "Lead", "PreRoll", "Preroll", "Prewarm", "Preheat", "Advance"}
	windowKeys := []string{"LookaheadFrames", "LookAheadFrames", "FrameLookahead", "LookaheadWindow", "WindowFrames", "FrameWindow", "WindowLength", "WindowSize", "Window", "HorizonFrames", "Horizon"}

	// Policy supplies unified defaults; schedule sections override them.
	if n, ok := intFromSources(leadKeys, r.Policy); ok {
		cfg.lead = n
	}
	if n, ok := intFromSources(windowKeys, r.Policy); ok {
		cfg.window = n
	}
	if n, ok := intFromSources(leadKeys, extraConfig); ok {
		cfg.lead = n
	}
	if n, ok := intFromSources(append([]string{"Lookahead"}, windowKeys...), extraConfig); ok {
		cfg.window = n
	}
	for _, c := range containers {
		if c == nil {
			continue
		}
		if n, ok := intFromSources(leadKeys, c); ok {
			cfg.lead = n
		}
		if n, ok := intFromSources(append([]string{"Lookahead"}, windowKeys...), c); ok {
			cfg.window = n
		}
		m, err := anyStringMap(c)
		if err == nil {
			for _, key := range []string{"Opportunities", "Cues", "Shots", "PhotoOps", "FramingOpportunities", "Vistas", "Setups", "Bookings", "Takes", "DoorShots", "CameraCues", "Items"} {
				if v, ok := m[key]; ok {
					appendCues(&cfg.cues, v)
				}
			}
			for k, v := range m {
				nk := normalizeKey(k)
				wanted := nk == "items" ||
					strings.Contains(nk, "opportunit") ||
					strings.Contains(nk, "cue") ||
					strings.Contains(nk, "shot") ||
					strings.Contains(nk, "setup") ||
					strings.Contains(nk, "booking") ||
					strings.Contains(nk, "take") ||
					strings.Contains(nk, "vista") ||
					strings.Contains(nk, "framing")
				if wanted {
					appendCues(&cfg.cues, v)
				}
			}
			for _, flagKey := range []string{"Enabled", "Active", "On"} {
				if flag, ok := m[flagKey]; ok {
					cfg.active = truthy(flag)
				}
			}
		}
	}
	for _, l := range listHolders {
		appendCues(&cfg.cues, l)
	}

	if !cfg.active {
		return cfg, nil
	}
	if cfg.lead < 1 {
		return cfg, fmt.Errorf("show schedule requires at least one warmup frame")
	}
	if cfg.window < 0 {
		return cfg, fmt.Errorf("show schedule lookahead window cannot be negative")
	}
	return cfg, nil
}
