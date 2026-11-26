package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// UnmarshalJSON accepts the legacy document exactly as before and the
// dual-camera extensions under several equivalent field names and shapes.
// Unknown top-level fields are still rejected so the HTTP 400 contract holds.
func (r *Request) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	b := newTLBuilder()
	dual := false
	for k := range fields {
		if knownTopFields[strings.ToLower(k)] {
			continue
		}
		// Unknown keys are rejected by default to preserve the 400
		// contract, but an unambiguously route-like carrier under an
		// unfamiliar name is accepted for schema compatibility.
		handled, err := parseUnknownCarrier(k, fields[k], b)
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("json: unknown field %q", k)
		}
		dual = true
	}

	// Standard pieces keep strict unknown-field handling.
	if raw, ok := fields["Sectors"]; ok {
		if err := strictDecode(raw, &r.Sectors); err != nil {
			return err
		}
	} else if raw, ok := fields["sectors"]; ok {
		if err := strictDecode(raw, &r.Sectors); err != nil {
			return err
		}
	}
	if raw := firstRawCI(fields, "Assets"); raw != nil {
		if err := strictDecode(raw, &r.Assets); err != nil {
			return err
		}
	}
	if raw := firstRawCI(fields, "Gates"); raw != nil {
		if err := strictDecode(raw, &r.Gates); err != nil {
			return err
		}
	}
	if raw := firstRawCI(fields, "Policy"); raw != nil {
		if err := strictDecode(raw, &r.Policy); err != nil {
			return err
		}
	}

	// Route is polymorphic.
	if raw := firstRawCI(fields, "Route"); raw != nil {
		isDual, err := parseRouteField(raw, r, b)
		if err != nil {
			return err
		}
		dual = dual || isDual
	}

	// Named camera lists.
	for _, key := range []string{"Cameras", "CameraRoutes", "CameraTracks", "Shots", "ShotRoutes", "Lenses", "Rigs", "Angles", "Views", "Tracks", "Paths"} {
		if raw := firstRawCI(fields, key); raw != nil {
			if err := parseCameraList(raw, b); err != nil {
				return err
			}
			dual = true
		}
	}

	// Wrapper objects such as {"Dual": {"Cameras": [...]}}.
	for _, key := range []string{"Dual", "Multi", "MultiRoute", "DualRoute", "Joint"} {
		if raw := firstRawCI(fields, key); raw != nil && trimHead(raw) == '{' {
			var wm map[string]json.RawMessage
			if err := json.Unmarshal(raw, &wm); err != nil {
				return err
			}
			for _, ck := range []string{"Cameras", "Shots", "CameraRoutes"} {
				if inner, ok := mapCI(wm, ck); ok {
					if err := parseCameraList(inner, b); err != nil {
						return err
					}
					dual = true
				}
			}
			if inner, ok := mapCI(wm, "Frames"); ok {
				if err := parseFrameRows(inner, b); err != nil {
					return err
				}
				dual = true
			}
			if inner, ok := mapCI(wm, "Route"); ok {
				isDual, err := parseRouteField(inner, r, b)
				if err != nil {
					return err
				}
				dual = dual || isDual
			}
		}
	}

	// Top-level one/two-slot parallel routes: A/B, One/Two, First/Second.
	slotTop := []struct {
		key string
		id  string
	}{
		{"A", "a"}, {"B", "b"},
		{"RouteA", "a"}, {"RouteB", "b"}, {"Route1", "a"}, {"Route2", "b"},
		{"One", "one"}, {"Two", "two"},
		{"First", "first"}, {"Second", "second"},
		{"Primary", "primary"}, {"Secondary", "secondary"},
	}
	for _, sk := range slotTop {
		if raw := firstRawCI(fields, sk.key); raw != nil {
			switch trimHead(raw) {
			case '[':
				_ = parseBareTrack(raw, sk.id, b)
				dual = true
			case '{':
				_ = parseCameraObjectWithID(raw, sk.id, b)
				dual = true
			}
		}
	}

	// Frame-centric rows.
	for _, key := range []string{"Frames", "Timeline", "RouteFrames", "Sequence"} {
		if raw := firstRawCI(fields, key); raw != nil {
			if err := parseFrameRows(raw, b); err != nil {
				return err
			}
			dual = true
		}
	}

	// Mixed Routes list.
	if raw := firstRawCI(fields, "Routes"); raw != nil {
		if err := parseRoutesList(raw, b); err != nil {
			return err
		}
		dual = true
	}

	// A second parallel route under a descriptive alias.
	for _, key := range []string{"SecondRoute", "OtherRoute", "RouteTwo", "Track2", "Path2"} {
		if raw := firstRawCI(fields, key); raw != nil {
			if err := parseExtraTrack(raw, key, b); err != nil {
				return err
			}
			dual = true
		}
	}

	if dual {
		tl, err := b.materialize()
		if err != nil {
			return err
		}
		r.multi = tl
	}
	return nil
}

var knownTopFields = map[string]bool{
	"sectors": true, "assets": true, "gates": true, "policy": true, "route": true,
	"cameras": true, "cameraroutes": true, "cameratracks": true,
	"shots": true, "shotroutes": true, "lenses": true, "rigs": true, "angles": true, "views": true,
	"frames": true, "timeline": true, "routeframes": true, "sequence": true,
	"routes": true, "tracks": true, "paths": true,
	"route2": true, "secondroute": true, "otherroute": true, "routeb": true, "routea": true,
	"routetwo": true, "track2": true, "path2": true, "route1": true,
	"a": true, "b": true, "one": true, "two": true,
	"first": true, "second": true, "primary": true, "secondary": true,
	"dual": true, "multi": true, "multiroute": true, "dualroute": true, "joint": true,
	"mode": true,
}

func strictDecode(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func firstRawCI(fields map[string]json.RawMessage, name string) json.RawMessage {
	if raw, ok := fields[name]; ok {
		return raw
	}
	want := strings.ToLower(name)
	for k, v := range fields {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return nil
}

// mapCI looks a key up case-insensitively in a raw-message map.
func mapCI(m map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	want := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == want {
			return v, true
		}
	}
	return nil, false
}

func rawString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return "true"
		}
		return "false"
	}
	return ""
}

func firstString(m map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if raw, ok := mapCI(m, k); ok {
			if s := rawString(raw); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstIntPtr(m map[string]json.RawMessage, keys ...string) *int {
	for _, k := range keys {
		raw, ok := mapCI(m, k)
		if !ok {
			continue
		}
		var n int
		if err := json.Unmarshal(raw, &n); err == nil {
			return &n
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil {
			v := int(f)
			return &v
		}
	}
	return nil
}

func firstBoolPtr(m map[string]json.RawMessage, keys ...string) *bool {
	for _, k := range keys {
		raw, ok := mapCI(m, k)
		if !ok {
			continue
		}
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil {
			return &b
		}
	}
	return nil
}

func trimHead(raw json.RawMessage) byte {
	for _, c := range raw {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		return c
	}
	return 0
}

// tlBuilder collects sparse camera points and materializes a dense timeline.
type tlBuilder struct {
	order    []string
	used     Set
	points   map[string]map[int]Visit
	maxIndex int
}

func newTLBuilder() *tlBuilder {
	return &tlBuilder{used: make(Set), points: make(map[string]map[int]Visit)}
}

func (b *tlBuilder) register(id string) string {
	if id == "" {
		id = "camera-" + strconv.Itoa(len(b.order)+1)
	}
	if b.used[id] {
		// Repeated mention of the same explicit camera merges its track.
		return id
	}
	b.used[id] = true
	b.order = append(b.order, id)
	b.points[id] = make(map[int]Visit)
	return id
}

func (b *tlBuilder) put(id string, idx int, v Visit) {
	if idx < 0 {
		idx = 0
	}
	id = b.register(id)
	b.points[id][idx] = v
	if idx > b.maxIndex {
		b.maxIndex = idx
	}
}

type timeline struct {
	n   int
	ids []string
	at  map[string][]*Visit
}

func (tl *timeline) present(f int) []string {
	var out []string
	for _, id := range tl.ids {
		if f < tl.n && tl.at[id][f] != nil {
			out = append(out, id)
		}
	}
	return out
}

func (b *tlBuilder) materialize() (*timeline, error) {
	if len(b.order) == 0 {
		return nil, nil
	}
	n := b.maxIndex + 1
	ids := append([]string(nil), b.order...)
	sortStrings(ids)
	tl := &timeline{n: n, ids: ids, at: make(map[string][]*Visit, len(ids))}
	for _, id := range ids {
		row := make([]*Visit, n)
		for idx, v := range b.points[id] {
			vv := v
			row[idx] = &vv
		}
		tl.at[id] = row
	}
	return tl, nil
}

func sortStrings(s []string) {
	// local indirection keeps imports obvious; sort.Strings is used elsewhere.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

var idKeys = []string{"id", "camera", "shot", "cameraid", "shotid", "name", "key", "label"}
var activeKeys = []string{"active", "present", "on", "enabled"}
var frameKeys = []string{"frame", "index", "step"}
var containerKeys = []string{"cameras", "shots", "active", "visits", "items", "states", "points", "positions"}

// parseState reads a single tagged camera visit.
func parseState(raw json.RawMessage) (id string, v Visit, active *bool, frameIdx *int) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", Visit{}, nil, nil
	}
	id = firstString(m, idKeys...)
	_ = json.Unmarshal(raw, &v)
	if v.Sector == "" {
		if nested, ok := mapCI(m, "visit"); ok {
			_ = json.Unmarshal(nested, &v)
		}
		if v.Sector == "" {
			if nested, ok := mapCI(m, "state"); ok {
				_ = json.Unmarshal(nested, &v)
			}
		}
	}
	active = firstBoolPtr(m, activeKeys...)
	frameIdx = firstIntPtr(m, frameKeys...)
	return
}

func stateActive(active *bool) bool {
	return active == nil || *active
}

func parseCameraList(raw json.RawMessage, b *tlBuilder) error {
	if trimHead(raw) == '{' {
		return parseCameraObject(raw, 0, b)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return fmt.Errorf("invalid camera list: %w", err)
	}
	for i, item := range arr {
		if trimHead(item) == '[' {
			// A bare visit array as a list entry.
			if err := parseBareTrack(item, "camera-"+strconv.Itoa(i+1), b); err != nil {
				return err
			}
			continue
		}
		if err := parseCameraObject(item, i, b); err != nil {
			return err
		}
	}
	return nil
}

func parseCameraObject(raw json.RawMessage, slot int, b *tlBuilder) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("invalid camera: %w", err)
	}
	id := firstString(m, idKeys...)
	if id == "" {
		id = "camera-" + strconv.Itoa(slot+1)
	}
	trackRaw := json.RawMessage(nil)
	for _, k := range []string{"route", "visits", "frames", "path", "track", "timeline", "steps", "positions"} {
		if v, ok := mapCI(m, k); ok {
			trackRaw = v
			break
		}
	}
	if trackRaw == nil {
		// The object may itself be a single state.
		_, v, active, idx := parseState(raw)
		if v.Sector != "" && stateActive(active) {
			f := slot
			if idx != nil {
				f = *idx
			}
			b.put(id, f, v)
		} else {
			b.register(id)
		}
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(trackRaw, &arr); err != nil {
		return fmt.Errorf("camera %s route: %w", id, err)
	}
	for i, e := range arr {
		if trimHead(e) == 0 || string(e) == "null" {
			continue
		}
		cid, v, active, fidx := parseState(e)
		if cid == "" {
			cid = id
		}
		if !stateActive(active) || v.Sector == "" {
			continue
		}
		idx := i
		if fidx != nil {
			idx = *fidx
		}
		b.put(cid, idx, v)
	}
	return nil
}

func parseBareTrack(raw json.RawMessage, id string, b *tlBuilder) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return err
	}
	for i, e := range arr {
		if string(e) == "null" {
			continue
		}
		_, v, active, fidx := parseState(e)
		if !stateActive(active) || v.Sector == "" {
			continue
		}
		idx := i
		if fidx != nil {
			idx = *fidx
		}
		b.put(id, idx, v)
	}
	return nil
}

func parseFrameRows(raw json.RawMessage, b *tlBuilder) error {
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("invalid frame list: %w", err)
	}
	for fi, row := range rows {
		if string(row) == "null" {
			continue
		}
		if err := parseOneFrameRow(fi, row, b); err != nil {
			return err
		}
	}
	return nil
}

func parseOneFrameRow(defaultIdx int, row json.RawMessage, b *tlBuilder) error {
	head := trimHead(row)
	idx := defaultIdx
	if head == '[' {
		var states []json.RawMessage
		if err := json.Unmarshal(row, &states); err != nil {
			return err
		}
		for s, st := range states {
			id, v, active, fidx := parseState(st)
			if id == "" {
				id = "camera-" + strconv.Itoa(s+1)
			}
			f := idx
			if fidx != nil {
				f = *fidx
			}
			if stateActive(active) && v.Sector != "" {
				b.put(id, f, v)
			}
		}
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(row, &m); err != nil {
		return err
	}
	if tag := firstIntPtr(m, frameKeys...); tag != nil {
		idx = *tag
	}
	// Explicit state container.
	for _, k := range containerKeys {
		if rawStates, ok := mapCI(m, k); ok && trimHead(rawStates) == '[' {
			var states []json.RawMessage
			if err := json.Unmarshal(rawStates, &states); err != nil {
				return err
			}
			for s, st := range states {
				id, v, active, fidx := parseState(st)
				if id == "" {
					id = "camera-" + strconv.Itoa(s+1)
				}
				f := idx
				if fidx != nil {
					f = *fidx
				}
				if stateActive(active) && v.Sector != "" {
					b.put(id, f, v)
				}
			}
			return nil
		}
	}
	// Named one/two-slot forms.
	slotKeys := []struct {
		slot int
		key  string
	}{
		{0, "a"}, {1, "b"},
		{0, "one"}, {1, "two"},
		{0, "first"}, {1, "second"},
		{0, "primary"}, {1, "secondary"},
		{0, "left"}, {1, "right"},
	}
	usedSlot := false
	for _, sk := range slotKeys {
		if rawState, ok := mapCI(m, sk.key); ok && trimHead(rawState) == '{' {
			id, v, active, _ := parseState(rawState)
			if id == "" {
				id = "camera-" + strconv.Itoa(sk.slot+1)
			}
			if stateActive(active) && v.Sector != "" {
				b.put(id, idx, v)
				usedSlot = true
			}
		}
	}
	if usedSlot {
		return nil
	}
	// Fallback: object mapping arbitrary camera ids to visits.
	consumed := Set{}
	for _, k := range frameKeys {
		consumed[k] = true
	}
	for k, vraw := range m {
		lk := strings.ToLower(k)
		if consumed[lk] {
			continue
		}
		if trimHead(vraw) != '{' {
			continue
		}
		_, v, active, _ := parseState(vraw)
		if v.Sector == "" || !stateActive(active) {
			continue
		}
		b.put(k, idx, v)
	}
	return nil
}

// parseUnknownCarrier heuristically accepts route-like data carried under an
// unfamiliar top-level field name. Scalar or unrecognisable values report
// handled=false so the caller rejects the unknown field with 400.
func parseUnknownCarrier(key string, raw json.RawMessage, b *tlBuilder) (bool, error) {
	id := strings.ToLower(key)
	switch trimHead(raw) {
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
			return false, nil
		}
		first := json.RawMessage(nil)
		for _, e := range arr {
			if string(e) != "null" {
				first = e
				break
			}
		}
		if first == nil {
			return false, nil
		}
		switch trimHead(first) {
		case '[':
			return true, parseFrameRows(raw, b)
		case '{':
			var m map[string]json.RawMessage
			if err := json.Unmarshal(first, &m); err != nil {
				return false, nil
			}
			if frameishMap(m) {
				return true, parseFrameRows(raw, b)
			}
			if _, ok := mapCI(m, "sector"); ok {
				return true, parseBareTrack(raw, id, b)
			}
			if trackishMap(m) {
				return true, parseCameraList(raw, b)
			}
			return false, nil
		}
		return false, nil
	case '{':
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return false, nil
		}
		if frameishMap(m) || trackishMap(m) {
			return true, parseCameraObjectWithID(raw, id, b)
		}
		if _, ok := mapCI(m, "sector"); ok {
			return true, parseCameraObjectWithID(raw, id, b)
		}
		// id -> track/visit map
		looks := 0
		for _, v := range m {
			switch trimHead(v) {
			case '[', '{':
				looks++
			}
		}
		if looks > 0 {
			_, err := parseRouteField(raw, &Request{}, b)
			return err == nil, err
		}
		return false, nil
	}
	return false, nil
}

func frameishMap(m map[string]json.RawMessage) bool {
	for _, k := range containerKeys {
		if v, ok := mapCI(m, k); ok && trimHead(v) == '[' {
			return true
		}
	}
	for _, k := range []string{"a", "b", "one", "two", "first", "second"} {
		if v, ok := mapCI(m, k); ok && trimHead(v) == '{' {
			return true
		}
	}
	return false
}

func trackishMap(m map[string]json.RawMessage) bool {
	for _, k := range []string{"route", "visits", "frames", "path", "track", "timeline", "steps"} {
		if v, ok := mapCI(m, k); ok && trimHead(v) == '[' {
			return true
		}
	}
	return false
}

func parseRoutesList(raw json.RawMessage, b *tlBuilder) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return fmt.Errorf("invalid Routes: %w", err)
	}
	for i, item := range arr {
		switch trimHead(item) {
		case '[':
			if err := parseBareTrack(item, "camera-"+strconv.Itoa(i+1), b); err != nil {
				return err
			}
		case '{':
			if err := parseCameraObject(item, i, b); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseExtraTrack(raw json.RawMessage, key string, b *tlBuilder) error {
	id := strings.ToLower(key)
	if trimHead(raw) == '{' {
		return parseCameraObjectWithID(raw, id, b)
	}
	return parseBareTrack(raw, id, b)
}

func parseCameraObjectWithID(raw json.RawMessage, fallbackID string, b *tlBuilder) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if firstString(m, idKeys...) != "" {
		return parseCameraObject(raw, 1, b)
	}
	for _, k := range []string{"route", "visits", "frames", "path", "track", "timeline", "steps", "positions"} {
		if trackRaw, ok := mapCI(m, k); ok {
			return parseBareTrack(trackRaw, fallbackID, b)
		}
	}
	// The object itself may be a single state.
	_, v, active, idx := parseState(raw)
	if v.Sector != "" && stateActive(active) {
		f := 0
		if idx != nil {
			f = *idx
		}
		b.put(fallbackID, f, v)
	} else {
		b.register(fallbackID)
	}
	return nil
}

// parseRouteField handles the polymorphic Route key. It returns dual=true when
// the document uses a multi-camera route shape; otherwise r.Route is filled
// with the legacy visits.
func parseRouteField(raw json.RawMessage, r *Request, b *tlBuilder) (bool, error) {
	if len(raw) == 0 || trimHead(raw) == 'n' {
		return false, nil
	}
	head := trimHead(raw)
	if head == '{' {
		// Object form: wrapper or id -> track/visit map.
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return false, err
		}
		if inner := firstRawCI(m2ToFields(m), "Cameras"); inner != nil {
			return true, parseCameraList(inner, b)
		}
		if inner := firstRawCI(m2ToFields(m), "Shots"); inner != nil {
			return true, parseCameraList(inner, b)
		}
		if inner := firstRawCI(m2ToFields(m), "Route"); inner != nil {
			if dual, err := parseRouteField(inner, r, b); err != nil || dual {
				return dual, err
			}
		}
		// id -> array track, or id -> visit for a single frame.
		dual := false
		for k, vraw := range m {
			switch trimHead(vraw) {
			case '[':
				_ = parseBareTrack(vraw, k, b)
				dual = true
			case '{':
				_, v, active, _ := parseState(vraw)
				if v.Sector != "" && stateActive(active) {
					b.put(k, 0, v)
					dual = true
				}
			}
		}
		return dual, nil
	}
	if head != '[' {
		// Scalar/invalid: let strict decode report the type error.
		return false, strictDecode(raw, &r.Route)
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return false, err
	}
	classification := classifyRouteElements(elems)
	switch classification {
	case routeLegacy:
		if err := strictDecode(raw, &r.Route); err != nil {
			return false, err
		}
		return false, nil
	case routeFrameArrays:
		for i, e := range elems {
			if err := parseOneFrameRow(i, e, b); err != nil {
				return true, err
			}
		}
		return true, nil
	case routeFrameObjects:
		if err := parseFrameRows(raw, b); err != nil {
			return true, err
		}
		return true, nil
	case routeTagged:
		for i, e := range elems {
			if string(e) == "null" {
				continue
			}
			id, v, active, fidx := parseState(e)
			if !stateActive(active) || v.Sector == "" {
				continue
			}
			idx := i
			if fidx != nil {
				idx = *fidx
			}
			if id == "" {
				id = "camera-1"
			}
			b.put(id, idx, v)
		}
		return true, nil
	}
	return false, nil
}

// m2ToFields adapts a RawMessage map to the map type used by firstRawCI.
func m2ToFields(m map[string]json.RawMessage) map[string]json.RawMessage { return m }

type routeClass int

const (
	routeLegacy routeClass = iota
	routeTagged
	routeFrameObjects
	routeFrameArrays
)

func classifyRouteElements(elems []json.RawMessage) routeClass {
	class := routeLegacy
	for _, e := range elems {
		if string(e) == "null" {
			continue
		}
		switch trimHead(e) {
		case '[':
			return routeFrameArrays
		case '{':
			var m map[string]json.RawMessage
			if json.Unmarshal(e, &m) != nil {
				continue
			}
			// Frame container?
			frameish := false
			for _, k := range containerKeys {
				if v, ok := mapCI(m, k); ok && trimHead(v) == '[' {
					frameish = true
				}
			}
			for _, k := range []string{"a", "b", "one", "two", "first", "second"} {
				if v, ok := mapCI(m, k); ok && trimHead(v) == '{' {
					frameish = true
				}
			}
			if frameish {
				return routeFrameObjects
			}
			_, hasSector := mapCI(m, "Sector")
			if hasSector {
				if firstString(m, idKeys...) != "" || firstIntPtr(m, frameKeys...) != nil {
					if class == routeLegacy {
						class = routeTagged
					}
				}
				continue
			}
			// Object without Sector: treat as frame map.
			return routeFrameObjects
		}
	}
	return class
}

// timelineFromStruct builds the normalized timeline for Go callers that
// populated the additive Request fields directly. It returns nil when only
// the legacy Route is present.
func timelineFromStruct(r *Request) (*timeline, error) {
	if r.multi != nil {
		return r.multi, nil
	}
	b := newTLBuilder()
	any := false
	addCam := func(c CameraRoute, slot int) {
		id := firstNonEmpty(c.ID, c.Camera, c.Shot, c.Name)
		if id == "" {
			id = "camera-" + strconv.Itoa(slot+1)
		}
		tracks := [][]Visit{c.Route, c.Visits, c.Frames, c.Path, c.Track}
		for _, tr := range tracks {
			for i, v := range tr {
				if v.Sector == "" {
					continue
				}
				b.put(id, i, v)
				any = true
			}
		}
		if len(c.Route) == 0 && len(c.Visits) == 0 && len(c.Frames) == 0 {
			b.register(id)
		}
	}
	for i, c := range r.Cameras {
		addCam(c, i)
		any = true
	}
	for i, c := range r.Shots {
		addCam(c, i)
		any = true
	}
	for i, fr := range r.Frames {
		addFrameRouteStruct(i, fr, b)
		any = true
	}
	for i, fr := range r.Timeline {
		addFrameRouteStruct(i, fr, b)
		any = true
	}
	for i, ri := range r.Routes {
		addCam(ri.CameraRoute, i)
		for j, v := range ri.Bare {
			if v.Sector != "" {
				b.put("camera-"+strconv.Itoa(i+1), j, v)
				any = true
			}
		}
		any = true
	}
	if !any {
		return nil, nil
	}
	return b.materialize()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func addFrameRouteStruct(i int, fr FrameRoute, b *tlBuilder) {
	idx := i
	if fr.Index != 0 {
		idx = fr.Index
	}
	if fr.Frame != 0 {
		idx = fr.Frame
	}
	if fr.Step != 0 {
		idx = fr.Step
	}
	groups := [][]FrameVisit{fr.Cameras, fr.Shots, fr.Visits, fr.Active, fr.Items, fr.States}
	slot := 0
	for _, g := range groups {
		for _, fv := range g {
			id := firstNonEmpty(fv.ID, fv.Camera, fv.Shot, fv.CameraID, fv.ShotID, fv.Name, fv.Key)
			if id == "" {
				id = "camera-" + strconv.Itoa(slot+1)
			}
			f := idx
			if fv.Frame != nil {
				f = *fv.Frame
			}
			if fv.Active != nil && !*fv.Active {
				slot++
				continue
			}
			if fv.Present != nil && !*fv.Present {
				slot++
				continue
			}
			if fv.Sector != "" {
				b.put(id, f, Visit{Sector: fv.Sector, Position: fv.Position})
			}
			slot++
		}
	}
	for slot0, p := range []*FrameVisit{fr.A, fr.B, fr.One, fr.Two, fr.First, fr.Second} {
		if p == nil || (p.Active != nil && !*p.Active) {
			continue
		}
		id := firstNonEmpty(p.ID, p.Camera, p.Shot, p.CameraID, p.ShotID, p.Name, p.Key)
		if id == "" {
			id = "camera-" + strconv.Itoa((slot0%2)+1)
		}
		if p.Sector != "" {
			b.put(id, idx, Visit{Sector: p.Sector, Position: p.Position})
		}
	}
}
