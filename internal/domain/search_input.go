package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Search is the normalized route-selection request.
type Search struct {
	Signposts []Signpost
	Moves     []Move
	Start     string
	End       string
	MaxMoves  int
	HasMax    bool
}

var signpostKeys = []string{
	"signposts", "signpost", "roadsigns", "roadsign", "roadmarks",
	"roadmark", "guideposts", "guidepost", "signboards", "signboard",
	"waypoints", "waypoint", "markers", "marker",
	"landmarks", "landmark", "beacons", "beacon", "nodes", "node", "points",
	"point", "stops", "stop", "cues", "cue", "keyframes", "keyframe",
	"controlpoints", "controls", "posts", "post", "anchors", "anchor",
	"vertices", "verts", "sites", "site",
}

var moveKeys = []string{
	"moves", "move", "allowedmoves", "legalmoves", "directedmoves",
	"directededges", "edges", "edge", "links", "link",
	"transitions", "transition", "arcs", "arc", "connections", "connection",
	"adjacency", "adjacencies", "directions", "paths", "path", "legs",
	"leg", "segments", "segment",
}

var startKeys = []string{
	"start", "startid", "startsignpost", "startsignpostid",
	"startwaypoint", "startwaypointid", "startmarker", "startnode",
	"startref", "startat", "startpoint", "startpointid", "from", "fromid",
	"origin", "source", "src", "entry", "entrypoint", "begin", "beginning",
	"first",
}

var endKeys = []string{
	"end", "endid", "endsignpost", "endsignpostid", "endwaypoint",
	"endmarker", "endnode", "endref", "endat", "endpoint", "endpointid",
	"to", "toid", "goal", "target", "destination", "dest", "finish",
	"terminus", "last", "final",
}

var maxKeys = []string{
	"maxmoves", "maximummoves", "maxmovecount", "movecountlimit",
	"moveslimit", "movelimit", "maxsteps", "stepcount", "maxstepcount",
	"steplimit", "stepbudget", "movebudget", "maxtransitions", "maxedges",
	"maxhops", "hops", "maxlegs", "maxsegments", "maxiterations",
	"maxmove", "maxstep", "limit", "max",
}

var containerKeys = []string{
	"search", "tour", "routesearch", "routerequest", "routeplan",
	"routeplanner", "planner", "navigation", "nav", "camera", "shot",
	"query", "intent", "task", "options", "params", "parameters", "input",
	"request", "routing", "findroute",
}

func allKnownSearchKeys() Set {
	s := make(Set)
	for _, group := range [][]string{signpostKeys, moveKeys, startKeys, endKeys, maxKeys, containerKeys} {
		for _, k := range group {
			s[k] = true
		}
	}
	return s
}

var knownSearchKeys = allKnownSearchKeys()

func searchKeyKnown(nk string) bool { return knownSearchKeys[nk] }

// tokenizeKey splits snake_case, kebab-case, dotted and CamelCase names into
// lower-cased words so compound aliases classify by whole tokens rather than
// fragile substrings ("Intended" must not match "end").
func tokenizeKey(key string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	runes := []rune(key)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			flush()
		case i > 0 && r >= 'A' && r <= 'Z' && runes[i-1] >= 'a' && runes[i-1] <= 'z':
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

func hasToken(tokens []string, want ...string) bool {
	for _, t := range tokens {
		for _, w := range want {
			if t == w {
				return true
			}
		}
	}
	return false
}

// classifyFallback recognizes unanticipated field names by their name and
// value shape. It returns one of signposts/moves/start/end/maxmoves or "".
func classifyFallback(key string, raw json.RawMessage) string {
	nk := normKey(key)
	if !rawPresent(raw) {
		return ""
	}
	trimmed := strings.TrimSpace(string(raw))
	first := trimmed[0]

	nameHit := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(nk, w) {
				return true
			}
		}
		return false
	}

	switch {
	case first == '[':
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
			return ""
		}
		e0 := strings.TrimSpace(string(arr[0]))
		if strings.HasPrefix(e0, "[") {
			return "moves"
		}
		if strings.HasPrefix(e0, "{") {
			var obj map[string]json.RawMessage
			if json.Unmarshal(arr[0], &obj) != nil {
				return ""
			}
			_, hasSector := lookupNorm(obj, "sector", "sectorid", "zone", "region", "area")
			_, hasPos := lookupNorm(obj, "position", "pos", "at", "coordinate", "coordinates", "x")
			_, hasFrom := lookupNorm(obj, "from", "source", "src", "origin", "a")
			_, hasTo := lookupNorm(obj, "to", "target", "dst", "destination", "b")
			if hasSector && hasPos {
				return "signposts"
			}
			if hasFrom && hasTo {
				return "moves"
			}
		}
		if nameHit("signpost", "waypoint", "marker", "landmark", "beacon", "node") {
			return "signposts"
		}
		if nameHit("move", "edge", "link", "transition", "arc") {
			return "moves"
		}
	case first == '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return ""
		}
		if nameHit("adjac", "move", "edge", "link") {
			return "moves"
		}
		// Map of signpost id -> definition?
		for _, v := range obj {
			var d map[string]json.RawMessage
			if json.Unmarshal(v, &d) == nil {
				_, hs := lookupNorm(d, "sector", "zone", "region")
				_, hp := lookupNorm(d, "position", "pos", "at", "x")
				if hs && hp {
					return "signposts"
				}
			}
		}
	case first == '"' || first >= '0' && first <= '9':
		tokens := tokenizeKey(key)
		isStart := hasToken(tokens, "start", "origin", "source", "src", "from", "entry", "begin", "beginning", "first", "initial")
		isEnd := hasToken(tokens, "end", "goal", "target", "dest", "destination", "finish", "terminus", "last", "final", "to")
		isMax := hasToken(tokens, "max", "maximum", "limit", "cap", "ceiling", "upto") ||
			(hasToken(tokens, "move", "moves", "step", "steps", "hop", "hops", "leg", "legs", "edge", "edges", "transition", "transitions") &&
				hasToken(tokens, "count", "num", "number", "total", "limit", "max", "maximum"))
		switch {
		case isStart && !isEnd && !isMax:
			return "start"
		case isEnd && !isStart && !isMax:
			return "end"
		case isMax:
			return "maxmoves"
		}
	}
	return ""
}

func lookupNorm(m map[string]json.RawMessage, names ...string) (json.RawMessage, bool) {
	for _, name := range names {
		want := normKey(name)
		for k, v := range m {
			if normKey(k) == want {
				return v, true
			}
		}
	}
	return nil, false
}

// candidateMaps returns the top-level document followed by any nested object
// that plausibly wraps search parameters.
func candidateMaps(top map[string]json.RawMessage) []map[string]json.RawMessage {
	out := []map[string]json.RawMessage{top}
	// Recognized containers first.
	for _, name := range containerKeys {
		for k, raw := range top {
			if normKey(k) != name || !rawPresent(raw) {
				continue
			}
			var m map[string]json.RawMessage
			if json.Unmarshal(raw, &m) == nil {
				out = append(out, m)
			}
		}
	}
	// Then every other nested object, one level deep, in key order for
	// deterministic behavior.
	extraKeys := make([]string, 0, len(top))
	for k, raw := range top {
		if !rawPresent(raw) || strings.TrimSpace(string(raw))[0] != '{' {
			continue
		}
		if normKey(k) == "" {
			continue
		}
		extraKeys = append(extraKeys, k)
	}
	sort.Slice(extraKeys, func(i, j int) bool { return normKey(extraKeys[i]) < normKey(extraKeys[j]) })
	for _, k := range extraKeys {
		raw := top[k]
		var m map[string]json.RawMessage
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		already := false
		for _, ex := range out[1:] {
			if sameRawMap(ex, m) {
				already = true
			}
		}
		if !already {
			out = append(out, m)
		}
	}
	return out
}

func sameRawMap(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if string(b[k]) != string(v) {
			return false
		}
	}
	return true
}

// findConceptRaw locates the raw value for a concept using explicit aliases
// and then shape/name fallback discovery.
func findConceptRaw(top map[string]json.RawMessage, concept string, aliases []string) json.RawMessage {
	maps := candidateMaps(top)
	for _, alias := range aliases {
		want := normKey(alias)
		for _, m := range maps {
			for k, v := range m {
				if normKey(k) == want && rawPresent(v) {
					return v
				}
			}
		}
	}
	// Fallback discovery within every candidate map.
	for _, m := range maps {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			nk := normKey(k)
			if searchKeyKnown(nk) {
				continue
			}
			if classifyFallback(k, v) == concept {
				return v
			}
		}
	}
	return nil
}

func parseIntFlex(raw json.RawMessage) (int, bool) {
	if !rawPresent(raw) {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 0 && s[0] == '"' {
		var str string
		if json.Unmarshal(raw, &str) == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(str)); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, true
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil && f == float64(int(f)) {
		return int(f), true
	}
	return 0, false
}

func parsePosition(raw json.RawMessage) (int, bool) {
	if n, ok := parseIntFlex(raw); ok {
		return n, true
	}
	if !rawPresent(raw) {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	switch s[0] {
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
			return parseIntFlex(arr[0])
		}
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil {
			if v, ok := lookupNorm(obj, "x", "position", "pos", "at", "value", "coordinate"); ok {
				return parsePosition(v)
			}
		}
	}
	return 0, false
}

func parseStringFlex(raw json.RawMessage) string {
	if !rawPresent(raw) {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	if dec := json.NewDecoder(strings.NewReader(string(raw))); dec != nil {
		dec.UseNumber()
		var v any
		if dec.Decode(&v) == nil {
			switch t := v.(type) {
			case json.Number:
				return t.String()
			}
		}
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

type rawRef struct {
	id    string
	index int
	hasIx bool
	visit *Visit
}

func parseRef(raw json.RawMessage) rawRef {
	rr := rawRef{}
	if !rawPresent(raw) {
		return rr
	}
	s := strings.TrimSpace(string(raw))
	switch s[0] {
	case '"':
		rr.id = parseStringFlex(raw)
		return rr
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-':
		if n, ok := parseIntFlex(raw); ok {
			rr.index, rr.hasIx = n, true
		}
		return rr
	case '[':
		// Tuple reference: [id, sector, position], [sector, position] or [id].
		var parts []json.RawMessage
		if json.Unmarshal(raw, &parts) == nil {
			var strs []string
			var pos int
			hasPos := false
			for _, p := range parts {
				t := strings.TrimSpace(string(p))
				if n, ok := parseIntFlex(p); ok && !hasPos && len(t) > 0 && (t[0] == '-' || (t[0] >= '0' && t[0] <= '9')) {
					pos, hasPos = n, true
					continue
				}
				if len(t) > 0 && t[0] == '"' {
					strs = append(strs, parseStringFlex(p))
				}
			}
			switch {
			case len(strs) >= 2 && hasPos:
				rr.id = strs[0]
				rr.visit = &Visit{Sector: strs[1], Position: pos}
			case len(strs) == 1 && hasPos:
				rr.visit = &Visit{Sector: strs[0], Position: pos}
			case len(strs) >= 1:
				rr.id = strs[0]
			}
		}
		return rr
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return rr
		}
		if v, ok := lookupNorm(obj, "index", "idx", "order"); ok {
			if n, yes := parseIntFlex(v); yes {
				rr.index, rr.hasIx = n, true
			}
		}
		if v, ok := lookupNorm(obj, "id", "signpostid", "waypointid", "markerid", "nodeid", "key", "signpost", "waypoint", "marker", "ref", "name", "code", "target", "value"); ok {
			rr.id = parseStringFlex(v)
		}
		secRaw, hasSec := lookupNorm(obj, "sector", "sectorid", "zone", "region")
		posRaw, hasPos := lookupNorm(obj, "position", "pos", "at", "x")
		if hasSec && hasPos {
			sec := parseStringFlex(secRaw)
			if pos, ok := parsePosition(posRaw); ok {
				rr.visit = &Visit{Sector: sec, Position: pos}
			}
		}
		return rr
	default:
		rr.id = parseStringFlex(raw)
		return rr
	}
}

func parseSignposts(raw json.RawMessage) []Signpost {
	if !rawPresent(raw) {
		return nil
	}
	s := strings.TrimSpace(string(raw))
	var out []Signpost
	if s[0] == '{' {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		ids := make([]string, 0, len(obj))
		for id := range obj {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			out = append(out, parseSignpostItem(obj[id], id, len(out)))
		}
		return out
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil {
		return nil
	}
	for i, it := range arr {
		out = append(out, parseSignpostItem(it, "", i))
	}
	return out
}

func parseSignpostItem(raw json.RawMessage, mapID string, index int) Signpost {
	sp := Signpost{}
	id := mapID
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		if id == "" {
			id = fmt.Sprintf("signpost-%d", index)
		}
		sp.ID = id
		return sp
	}
	first := trimmed[0]
	switch {
	case first == '[':
		// Tuple forms: [id, sector, position] or [sector, position].
		var parts []json.RawMessage
		if json.Unmarshal(raw, &parts) == nil {
			var strings2 []string
			for _, p := range parts {
				t := strings.TrimSpace(string(p))
				if n, ok := parseIntFlex(p); ok && sp.Position == 0 && (t[0] == '-' || (t[0] >= '0' && t[0] <= '9')) {
					sp.Position = n
					continue
				}
				if t[0] == '"' {
					strings2 = append(strings2, parseStringFlex(p))
				}
			}
			if mapID == "" && len(strings2) >= 2 {
				id = strings2[0]
				sp.Sector = strings2[1]
			} else if len(strings2) >= 1 {
				sp.Sector = strings2[len(strings2)-1]
			}
		}
	case first == '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil {
			if v, ok := lookupNorm(obj, "id", "key", "name", "ref", "label", "code"); ok {
				if s := parseStringFlex(v); s != "" {
					id = s
				}
			}
			if v, ok := lookupNorm(obj, "sector", "sectorid", "zone", "region", "area"); ok {
				sp.Sector = parseStringFlex(v)
			}
			if v, ok := lookupNorm(obj, "position", "pos", "at", "coordinate", "coordinates", "x"); ok {
				sp.Position, _ = parsePosition(v)
			}
		}
	default:
		// Bare string / number signpost reference.
		id = parseStringFlex(raw)
	}
	if id == "" {
		id = fmt.Sprintf("signpost-%d", index)
	}
	sp.ID = id
	return sp
}

type rawMove struct {
	from rawRef
	to   rawRef
	gate string
}

func parseMoves(raw json.RawMessage) []rawMove {
	if !rawPresent(raw) {
		return nil
	}
	s := strings.TrimSpace(string(raw))
	var out []rawMove
	switch s[0] {
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil {
			return nil
		}
		for _, it := range arr {
			t := strings.TrimSpace(string(it))
			if strings.HasPrefix(t, "{") {
				var obj map[string]json.RawMessage
				if json.Unmarshal(it, &obj) != nil {
					continue
				}
				rm := rawMove{}
				if v, ok := lookupNorm(obj, "from", "fromid", "source", "src", "origin", "a", "start"); ok {
					rm.from = parseRef(v)
				}
				if v, ok := lookupNorm(obj, "to", "toid", "target", "dst", "destination", "b", "end"); ok {
					rm.to = parseRef(v)
				}
				if v, ok := lookupNorm(obj, "gate", "gateid", "door", "portal"); ok {
					rm.gate = parseStringFlex(v)
				}
				out = append(out, rm)
			} else if strings.HasPrefix(t, "[") {
				var pair []json.RawMessage
				if json.Unmarshal(it, &pair) != nil || len(pair) < 2 {
					continue
				}
				rm := rawMove{from: parseRef(pair[0]), to: parseRef(pair[1])}
				if len(pair) >= 3 {
					rm.gate = parseStringFlex(pair[2])
				}
				out = append(out, rm)
			} else {
				// "a -> b" shorthand.
				text := parseStringFlex(it)
				parts := splitArrow(text)
				if len(parts) >= 2 {
					rm := rawMove{from: rawRef{id: parts[0]}, to: rawRef{id: parts[1]}}
					if len(parts) >= 3 {
						rm.gate = parts[2]
					}
					out = append(out, rm)
				}
			}
		}
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			return nil
		}
		froms := make([]string, 0, len(obj))
		for k := range obj {
			froms = append(froms, k)
		}
		sort.Strings(froms)
		for _, f := range froms {
			v := obj[f]
			t := strings.TrimSpace(string(v))
			switch {
			case strings.HasPrefix(t, "["):
				var tos []json.RawMessage
				if json.Unmarshal(v, &tos) != nil {
					continue
				}
				for _, to := range tos {
					rm := rawMove{from: rawRef{id: f}}
					tt := strings.TrimSpace(string(to))
					if strings.HasPrefix(tt, "{") {
						rm.to = parseRef(to)
						var o map[string]json.RawMessage
						if json.Unmarshal(to, &o) == nil {
							if g, ok := lookupNorm(o, "gate", "gateid"); ok {
								rm.gate = parseStringFlex(g)
							}
						}
					} else {
						rm.to = parseRef(to)
					}
					out = append(out, rm)
				}
			default:
				rm := rawMove{from: rawRef{id: f}, to: parseRef(v)}
				out = append(out, rm)
			}
		}
	}
	return out
}

func splitArrow(s string) []string {
	for _, sep := range []string{"->", "→", "=>", ","} {
		if strings.Contains(s, sep) {
			parts := strings.Split(s, sep)
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return parts
		}
	}
	return strings.Fields(s)
}

// buildSearch normalizes all request wire shapes into a Search. It returns a
// domain error for authored values that cannot be resolved.
func buildSearch(r Request) (Search, bool, error) {
	top := r.raw
	if top == nil {
		// Direct in-process construction; merge source-level aliases.
		sps := r.Signposts
		if len(sps) == 0 {
			sps = r.Waypoints
		}
		if len(sps) == 0 {
			sps = r.Markers
		}
		mvs := r.Moves
		if len(mvs) == 0 {
			mvs = r.Edges
		}
		end := r.End
		if end == "" {
			end = r.Goal
		}
		maxMoves := r.MaxMoves
		hasMax := r.MaxMoves > 0
		if maxMoves == 0 && r.MaxSteps > 0 {
			maxMoves, hasMax = r.MaxSteps, true
		}
		if len(sps) == 0 && len(mvs) == 0 && r.Start == "" && end == "" {
			return Search{}, false, nil
		}
		if !hasMax {
			maxMoves = 2048
		}
		return Search{
			Signposts: sps,
			Moves:     mvs,
			Start:     r.Start,
			End:       end,
			MaxMoves:  maxMoves,
			HasMax:    hasMax,
		}, true, nil
	}

	spRaw := findConceptRaw(top, "signposts", signpostKeys)
	mvRaw := findConceptRaw(top, "moves", moveKeys)
	startRaw := findConceptRaw(top, "start", startKeys)
	endRaw := findConceptRaw(top, "end", endKeys)
	maxRaw := findConceptRaw(top, "maxmoves", maxKeys)

	signposts := parseSignposts(spRaw)
	rawMoves := parseMoves(mvRaw)

	// Inline adjacency declared on signpost objects.
	if rawPresent(spRaw) {
		collectInlineMoves(spRaw, &rawMoves)
	}

	// Inline start/end coordinates become synthetic signposts.
	startRef := parseRef(startRaw)
	endRef := parseRef(endRaw)
	signposts = appendInlineRef(signposts, &startRef, "start")
	signposts = appendInlineRef(signposts, &endRef, "end")

	idByIndex := make([]string, len(signposts))
	indexByID := make(map[string]int, len(signposts))
	for i, sp := range signposts {
		idByIndex[i] = sp.ID
		if _, dup := indexByID[sp.ID]; !dup {
			indexByID[sp.ID] = i
		}
	}

	resolve := func(rr rawRef, what string) (string, error) {
		if rr.visit != nil && rr.id != "" {
			return rr.id, nil
		}
		if rr.id != "" {
			if _, ok := indexByID[rr.id]; ok {
				return rr.id, nil
			}
			if n, err := strconv.Atoi(rr.id); err == nil && rr.hasIx == false {
				if n >= 0 && n < len(idByIndex) {
					return idByIndex[n], nil
				}
			}
		}
		if rr.hasIx {
			if rr.index < 0 || rr.index >= len(idByIndex) {
				return "", fmt.Errorf("%s signpost index %d out of range", what, rr.index)
			}
			return idByIndex[rr.index], nil
		}
		if rr.id != "" {
			return "", fmt.Errorf("%s signpost %q is not defined", what, rr.id)
		}
		return "", fmt.Errorf("%s signpost is missing", what)
	}

	startID, err := resolve(startRef, "start")
	if err != nil {
		return Search{}, false, err
	}
	endID, err := resolve(endRef, "end")
	if err != nil {
		return Search{}, false, err
	}

	moves := make([]Move, 0, len(rawMoves))
	for i, rm := range rawMoves {
		f, err := resolve(rm.from, fmt.Sprintf("move %d from", i))
		if err != nil {
			return Search{}, false, err
		}
		t, err := resolve(rm.to, fmt.Sprintf("move %d to", i))
		if err != nil {
			return Search{}, false, err
		}
		moves = append(moves, Move{From: f, To: t, Gate: rm.gate})
	}

	maxMoves := 0
	hasMax := false
	if n, ok := parseIntFlex(maxRaw); ok {
		maxMoves, hasMax = n, true
	}
	if !hasMax {
		switch {
		case r.Policy.MaxMoves != nil:
			maxMoves, hasMax = *r.Policy.MaxMoves, true
		case r.Policy.MaxMovesSnake != nil:
			maxMoves, hasMax = *r.Policy.MaxMovesSnake, true
		case r.Policy.MaxSteps != nil:
			maxMoves, hasMax = *r.Policy.MaxSteps, true
		}
	}
	if !hasMax {
		maxMoves = 2048
	}

	return Search{
		Signposts: signposts,
		Moves:     moves,
		Start:     startID,
		End:       endID,
		MaxMoves:  maxMoves,
		HasMax:    hasMax,
	}, true, nil
}

func appendInlineRef(signposts []Signpost, rr *rawRef, idHint string) []Signpost {
	if rr == nil || rr.visit == nil {
		return signposts
	}
	id := rr.id
	if id == "" {
		id = idHint
		for i := 0; ; i++ {
			candidate := id
			if i > 0 {
				candidate = fmt.Sprintf("%s-%d", idHint, i)
			}
			exists := false
			for _, sp := range signposts {
				if sp.ID == candidate {
					exists = true
					break
				}
			}
			if !exists {
				id = candidate
				break
			}
		}
	}
	rr.id = id
	return append(signposts, Signpost{ID: id, Sector: rr.visit.Sector, Position: rr.visit.Position})
}

func collectInlineMoves(spRaw json.RawMessage, rawMoves *[]rawMove) {
	s := strings.TrimSpace(string(spRaw))
	if s[0] != '[' {
		return
	}
	var arr []json.RawMessage
	if json.Unmarshal(spRaw, &arr) != nil {
		return
	}
	for idx, it := range arr {
		if strings.TrimSpace(string(it))[0] != '{' {
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(it, &obj) != nil {
			continue
		}
		id := ""
		if v, ok := lookupNorm(obj, "id", "key", "name", "ref"); ok {
			id = parseStringFlex(v)
		}
		if id == "" {
			id = fmt.Sprintf("signpost-%d", idx)
		}
		for _, keyName := range []string{"next", "to", "moves", "edges", "links", "transitions", "neighbors", "neighbours", "adjacency"} {
			v, ok := lookupNorm(obj, keyName)
			if !ok || !rawPresent(v) {
				continue
			}
			t := strings.TrimSpace(string(v))
			if strings.HasPrefix(t, "[") {
				var tos []json.RawMessage
				if json.Unmarshal(v, &tos) != nil {
					continue
				}
				for _, to := range tos {
					rm := rawMove{from: rawRef{id: id}}
					if strings.TrimSpace(string(to))[0] == '{' {
						var o map[string]json.RawMessage
						if json.Unmarshal(to, &o) == nil {
							if tv, ok := lookupNorm(o, "to", "target", "id", "signpost"); ok {
								rm.to = parseRef(tv)
							}
							if gv, ok := lookupNorm(o, "gate", "gateid"); ok {
								rm.gate = parseStringFlex(gv)
							}
						}
					} else {
						rm.to = parseRef(to)
					}
					*rawMoves = append(*rawMoves, rm)
				}
			}
		}
	}
}

func resolveRouteRefs(refs []string, signposts []Signpost) ([]Visit, error) {
	byID := make(map[string]Signpost, len(signposts))
	for _, sp := range signposts {
		byID[sp.ID] = sp
	}
	out := make([]Visit, 0, len(refs))
	for i, ref := range refs {
		if sp, ok := byID[ref]; ok {
			out = append(out, Visit{Sector: sp.Sector, Position: sp.Position})
			continue
		}
		if n, err := strconv.Atoi(ref); err == nil && n >= 0 && n < len(signposts) {
			sp := signposts[n]
			out = append(out, Visit{Sector: sp.Sector, Position: sp.Position})
			continue
		}
		return nil, fmt.Errorf("route reference %d %q is not a defined signpost", i, ref)
	}
	return out, nil
}
