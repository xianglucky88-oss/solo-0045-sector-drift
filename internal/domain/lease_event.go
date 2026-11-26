package domain

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

const (
	EventAcquire = "acquire"
	EventEnd     = "end"
)

// LeaseEvent is one runtime report for a single credential.
type LeaseEvent struct {
	Frame      int
	HasFrame   bool
	Credential string
	Kind       string // EventAcquire or EventEnd

	// Optional inline declaration carried by the event itself.
	Roots    []string
	Until    int
	HasUntil bool

	raw map[string]json.RawMessage
}

// EventBucket groups all events received for one frame.
type EventBucket struct {
	Frame  int
	Events []LeaseEvent
}

// UnmarshalJSON routes event decoding through the tolerant alias parser.
func (ev *LeaseEvent) UnmarshalJSON(data []byte) error {
	parsed := parseEventItem(data, 0)
	if len(parsed) > 0 {
		*ev = parsed[0]
	}
	return nil
}

// ReplayRequest is the tolerant wire input for a lease replay. It can reference
// an archived artifact by Digest, embed the downloaded Artifact/Plan directly,
// or embed the original build Request (which is recompiled). Events may be
// flat (each carrying a frame) or grouped into per-frame buckets.
type ReplayRequest struct {
	Digest  string
	Plan    *Plan
	Request *Request
	Leases  []Lease

	// Flat events; frame may live on each event.
	Events []LeaseEvent

	// Bucketed frames.
	Buckets   []EventBucket
	HasFrames bool

	Through    int
	HasThrough bool

	// Session stepping hint.
	Frame    int
	HasFrame bool
}

var digestKeys = []string{
	"Digest", "digest", "ArtifactDigest", "artifact_digest", "SHA256", "sha256",
	"Hash", "hash", "ID", "Id", "id", "Ref", "ref", "ArtifactID", "artifact_id",
}

var planKeys = []string{
	"Artifact", "artifact", "Plan", "plan", "StreamPlan", "stream_plan",
	"Product", "product", "Blob", "blob", "Download", "download",
}

var requestKeys = []string{
	"Request", "request", "Spec", "spec", "Input", "input", "Build", "build",
	"Source", "source", "Definition", "definition", "Document", "document",
}

var leaseCollectionKeys = []string{
	"Leases", "leases", "LeaseDeclarations", "lease_declarations", "LeaseDecls",
	"Tokens", "tokens", "Credentials", "credentials", "Holds", "holds",
	"Vouchers", "vouchers", "Permits", "permit", "Tickets", "tickets",
	"Cutscenes", "cutscenes", "References", "references",
}

var flatEventKeys = []string{
	"Events", "events", "EventLog", "event_log", "Timeline", "timeline",
	"Trace", "trace", "Log", "log", "Items", "items", "Results", "results",
	"Operations2", "RuntimeEvents", "runtime_events", "Signals", "signals",
	"Actual", "ActualEvents", "actual_events", "Live", "LiveEvents",
	"Runtime", "Callbacks", "Notifications", "Messages", "Feed",
	"Acquisitions", "LeaseEvents", "lease_events",
}

var bucketKeys = []string{
	"Frames", "frames", "Steps", "steps", "Ticks", "ticks",
	"FrameEvents", "frame_events", "EventFrames", "event_frames",
	"Buckets", "buckets", "PerFrame", "per_frame", "ByFrame", "by_frame",
}

func (r *ReplayRequest) UnmarshalJSON(data []byte) error {
	data = stripWS(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	// Top-level array: events, buckets, or array-of-arrays.
	if data[0] == '[' {
		var raws []json.RawMessage
		if err := json.Unmarshal(data, &raws); err == nil && len(raws) > 0 {
			first := bytes.TrimSpace(raws[0])
			switch {
			case len(first) > 0 && first[0] == '[':
				for i, raw := range raws {
					evs, _ := tryParseEventArray(raw, i)
					r.Buckets = append(r.Buckets, EventBucket{Frame: i, Events: evs})
				}
				r.HasFrames = true
				return nil
			case allBucketObjects(raws):
				var buckets []EventBucket
				if err := json.Unmarshal(data, &buckets); err == nil {
					r.Buckets, r.HasFrames = buckets, true
					return nil
				}
			}
		}
		if evs, n := parseEventList(data, 0); n > 0 {
			r.Events = evs
			return nil
		}
		// Fall back to buckets even if some frames are empty.
		if buckets, good := tryParseBuckets(data); good {
			r.Buckets, r.HasFrames = buckets, true
			return nil
		}
		if evs, _ := tryParseEventArray(data, 0); len(evs) > 0 {
			r.Events = evs
			return nil
		}
		return nil
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if v, ok := firstRaw(m, digestKeys); ok {
		r.Digest = digestFromRef(firstStringAny(m, digestKeys))
		_ = v
	}
	// Artifact may be a path like /v1/artifacts/<digest>.
	for _, k := range []string{"Artifact", "artifact", "ArtifactPath", "Location", "location", "Href", "href", "URL", "url"} {
		if s := firstStringAny(m, []string{k}); s != "" {
			if d := digestFromRef(s); d != "" {
				r.Digest = d
			}
		}
	}

	if v, ok := firstRaw(m, planKeys); ok {
		var p Plan
		if err := json.Unmarshal(v, &p); err == nil && (p.Schema != "" || len(p.Frames) > 0 || p.Budget > 0) {
			r.Plan = &p
		} else {
			// Maybe the artifact is a JSON string.
			var s string
			if err2 := json.Unmarshal(v, &s); err2 == nil && s != "" {
				var p2 Plan
				if err3 := json.Unmarshal([]byte(s), &p2); err3 == nil {
					r.Plan = &p2
				}
			}
		}
	}

	if v, ok := firstRaw(m, requestKeys); ok {
		var q Request
		if err := json.Unmarshal(v, &q); err == nil {
			r.Request = &q
		}
	}

	if v, ok := firstRaw(m, leaseCollectionKeys); ok {
		var ll leaseList
		if err := json.Unmarshal(v, &ll); err == nil {
			r.Leases = append(r.Leases, ll...)
		}
	}

	// If the body itself carries world definition fields, treat it as Request.
	if r.Request == nil {
		if _, hasSectors := m["Sectors"]; hasSectors {
			var q Request
			if err := json.Unmarshal(data, &q); err == nil {
				r.Request = &q
			}
		}
	}
	// If the body itself looks like a Plan, use it.
	if r.Plan == nil {
		if _, hasSchema := m["Schema"]; hasSchema {
			if _, hasFrames := m["Frames"]; hasFrames {
				var p Plan
				if err := json.Unmarshal(data, &p); err == nil {
					r.Plan = &p
				}
			}
		}
	}

	// Flat event lists.
	for _, k := range flatEventKeys {
		if v, ok := m[k]; ok {
			r.Events = append(r.Events, parseEventsValue(v, 0)...)
		}
	}

	// Bucketed frame lists. The same key may instead carry a flat event list
	// with frame indices on each event, so harvest both interpretations.
	for _, k := range bucketKeys {
		v, ok := m[k]
		if !ok {
			continue
		}
		if evs, n := parseEventList(v, 0); n > 0 {
			r.Events = append(r.Events, evs...)
		}
		if buckets, good := tryParseBuckets(v); good {
			total := 0
			for _, b := range buckets {
				total += len(b.Events)
			}
			if total > 0 {
				r.Buckets = append(r.Buckets, buckets...)
				r.HasFrames = true
			}
		}
	}

	// Frame-scoped acquire/end token lists directly on the body.
	if evs := parseTokenListFrame(m, frameFromMap(m, 0)); len(evs) > 0 {
		r.Events = append(r.Events, evs...)
	}

	if n, ok := firstRaw(m, []string{"Through", "through", "Until", "MaxFrame", "Frames_count", "Count", "FrameCount"}); ok {
		if x, err := decodeFlexInt(n); err == nil {
			r.Through, r.HasThrough = x, true
		}
	}
	if x := frameFromMap(m, -1); x >= 0 {
		r.Frame, r.HasFrame = x, true
	}

	// The body may itself be one event object rather than an envelope.
	if len(r.Events) == 0 && len(r.Buckets) == 0 && looksLikeStandaloneEvent(m) {
		def := 0
		if r.HasFrame {
			def = r.Frame
		}
		if evs := parseEventItem(data, def); len(evs) > 0 {
			r.Events = evs
		}
	}

	// Catch-all: unanticipated key names carrying explicit event objects.
	r.Events = append(r.Events, catchExtraEvents(m)...)
	return nil
}

// catchExtraEvents scans unknown array fields for objects that carry an
// explicit acquire/end verb, so events under unfamiliar keys are not lost.
func catchExtraEvents(m map[string]json.RawMessage) []LeaseEvent {
	skip := make(map[string]bool)
	for _, list := range [][]string{
		digestKeys, planKeys, requestKeys, leaseCollectionKeys,
		flatEventKeys, bucketKeys, credentialKeys,
		{"Sectors", "Assets", "Gates", "Route", "Policy", "Leases", "Frames",
			"Crossings", "Usage", "Pinned", "Decisions", "Operations",
			"Required", "Resident", "Residents", "Loaded", "Ops", "Actions"},
	} {
		for _, k := range list {
			skip[k] = true
			skip[normKey(k)] = true
		}
	}
	var out []LeaseEvent
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if skip[k] || skip[normKey(k)] {
			continue
		}
		var arr []json.RawMessage
		if json.Unmarshal(m[k], &arr) != nil {
			continue
		}
		for _, item := range arr {
			var mm map[string]json.RawMessage
			if json.Unmarshal(item, &mm) != nil {
				continue
			}
			if hasExplicitVerb(mm) {
				out = append(out, parseEventItem(item, 0)...)
			}
		}
	}
	return out
}

func hasExplicitVerb(m map[string]json.RawMessage) bool {
	if s := firstStringAny(m, kindKeys); s != "" && normalizeKind(s) != "" {
		return true
	}
	for _, k := range []string{
		"Acquire", "Acquired", "DoAcquire", "Gain", "Obtain",
		"End", "Ended", "Release", "Released", "Expire", "Expired",
	} {
		if b, ok := boolTrue(m[k]); ok && b {
			return true
		}
	}
	return len(parseTokenListFrame(m, frameFromMap(m, 0))) > 0
}

// looksLikeStandaloneEvent requires a recognized credential key and no
// envelope/world-definition keys, avoiding treating a digest document as event.
func looksLikeStandaloneEvent(m map[string]json.RawMessage) bool {
	if firstStringAny(m, credentialKeys) == "" {
		return false
	}
	for _, k := range []string{"Leases", "leases", "LeaseDeclarations", "Sectors", "Assets", "Route", "Artifact", "Plan", "Spec", "Request"} {
		if _, ok := m[k]; ok {
			return false
		}
	}
	return true
}

// DigestFromRef extracts a hex digest from a bare digest or an artifact path.
func DigestFromRef(s string) string { return digestFromRef(s) }

func digestFromRef(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 64 {
		return s
	}
	if i := strings.LastIndex(s, "/artifacts/"); i >= 0 {
		tail := s[i+len("/artifacts/"):]
		if j := strings.IndexAny(tail, "/?"); j >= 0 {
			tail = tail[:j]
		}
		return tail
	}
	if j := strings.IndexAny(s, "/?"); j >= 0 {
		s = s[:j]
	}
	return s
}

func tryParseEventArray(raw json.RawMessage, defaultFrame int) ([]LeaseEvent, bool) {
	evs, n := parseEventList(raw, defaultFrame)
	return evs, n >= 0
}

// parseEventList parses an array and returns the events plus the number of
// raw items that yielded at least one event.
func parseEventList(raw json.RawMessage, defaultFrame int) ([]LeaseEvent, int) {
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, -1
	}
	out := make([]LeaseEvent, 0, len(raws))
	hit := 0
	for _, item := range raws {
		evs := parseEventItem(item, defaultFrame)
		if len(evs) > 0 {
			hit++
		}
		out = append(out, evs...)
	}
	return out, hit
}

// parseEventsValue accepts an event array, a single event object, or an object
// carrying Acquire/End token lists.
func parseEventsValue(raw json.RawMessage, defaultFrame int) []LeaseEvent {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '[' {
		evs, _ := parseEventList(raw, defaultFrame)
		return evs
	}
	if trimmed[0] == '{' {
		return parseEventItem(raw, defaultFrame)
	}
	return nil
}

// allBucketObjects reports whether every object looks like a per-frame bucket
// rather than a single event.
func allBucketObjects(raws []json.RawMessage) bool {
	seen := 0
	for _, raw := range raws {
		it := bytes.TrimSpace(raw)
		if len(it) == 0 || it[0] != '{' {
			return false
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(it, &m) != nil {
			return false
		}
		if firstStringAny(m, credentialKeys) != "" {
			return false
		}
		looksBucket := false
		for _, k := range append(append([]string{}, flatEventKeys...), "FrameEvents", "EventFrames") {
			if _, ok := m[k]; ok {
				looksBucket = true
			}
		}
		if len(parseTokenListFrame(m, frameFromMap(m, 0))) > 0 {
			looksBucket = true
		}
		if !looksBucket {
			return false
		}
		seen++
	}
	return seen > 0
}

var kindKeys = []string{
	"Type", "type", "Kind", "kind", "Action", "action", "Event", "event",
	"Verb", "verb", "Name", "name", "Op", "op", "Status", "status",
	"Transition", "transition",
}

var acquireWords = map[string]bool{
	"acquire": true, "acquired": true, "acquiring": true, "obtain": true,
	"obtained": true, "gain": true, "gained": true, "got": true, "get": true,
	"start": true, "started": true, "begin": true, "began": true, "enter": true,
	"entered": true, "load": true, "loaded": true, "mount": true, "mounted": true,
	"hold": true, "held": true, "lease": true, "leased": true, "grant": true,
	"granted": true, "on": true, "add": true, "added": true, "request": true,
	"activate": true, "activated": true, "pin": true, "pinned": true, "take": true,
}

var endWords = map[string]bool{
	"end": true, "ended": true, "ending": true, "finish": true, "finished": true,
	"release": true, "released": true, "releasing": true, "close": true, "closed": true,
	"stop": true, "stopped": true, "cancel": true, "canceled": true, "cancelled": true,
	"expire": true, "expired": true, "expiry": true, "complete": true, "completed": true,
	"unload": true, "unloaded": true, "evict": true, "evicted": true, "off": true,
	"remove": true, "removed": true, "drop": true, "dropped": true, "free": true,
	"terminate": true, "terminated": true, "done": true, "clear": true, "cleared": true,
}

func normalizeKind(s string) string {
	k := normKey(s)
	switch {
	case acquireWords[k], strings.HasPrefix(k, "acqu"), strings.Contains(k, "acquire"):
		return EventAcquire
	case endWords[k], strings.Contains(k, "release"), strings.Contains(k, "end"),
		strings.Contains(k, "expir"), strings.Contains(k, "finish"):
		return EventEnd
	}
	return ""
}

func parseEventItem(raw json.RawMessage, defaultFrame int) []LeaseEvent {
	raw = stripWS(raw)
	if len(raw) == 0 {
		return nil
	}
	// Bare string event: "+token" acquire, "-token" end, otherwise acquire.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			s = strings.TrimSpace(s)
			switch {
			case strings.HasPrefix(s, "+"):
				return []LeaseEvent{{Frame: defaultFrame, Credential: s[1:], Kind: EventAcquire}}
			case strings.HasPrefix(s, "-"):
				return []LeaseEvent{{Frame: defaultFrame, Credential: s[1:], Kind: EventEnd}}
			default:
				return []LeaseEvent{{Frame: defaultFrame, Credential: s, Kind: EventAcquire}}
			}
		}
	}
	// Numeric? ignore.
	if raw[0] != '{' {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	frame := defaultFrame
	hasFrame := false
	if v, ok := firstRaw(m, frameKeys); ok {
		if n, err := decodeFlexInt(v); err == nil {
			frame, hasFrame = n, true
		}
	}
	if !hasFrame {
		// Unfamiliar frame key: prefer an int-named key containing frame-ish
		// fragments, otherwise the lone integer scalar.
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var lone *int
		count := 0
		for _, k := range keys {
			rv := bytes.TrimSpace(m[k])
			if len(rv) == 0 || rv[0] == '"' {
				continue
			}
			n, err := decodeFlexInt(rv)
			if err != nil {
				continue
			}
			nk := normKey(k)
			if strings.Contains(nk, "frame") || strings.Contains(nk, "step") ||
				strings.Contains(nk, "tick") || strings.Contains(nk, "index") || nk == "at" {
				frame, hasFrame = n, true
				break
			}
			count++
			vv := n
			lone = &vv
		}
		if !hasFrame && count == 1 && lone != nil {
			frame, hasFrame = *lone, true
		}
	}
	_ = frameFromMap

	// Expanded token-list form: {"Acquire": ["t1","t2"], "End": "t3"}.
	if evs := parseTokenListFrame(m, frame); len(evs) > 0 && firstStringAny(m, credentialKeys) == "" {
		for i := range evs {
			evs[i].HasFrame = hasFrame
		}
		return evs
	}

	cred := firstStringAny(m, credentialKeys)
	kind := ""
	if s := firstStringAny(m, kindKeys); s != "" {
		kind = normalizeKind(s)
	}
	// Boolean/semantic keys.
	if kind == "" {
		for _, k := range []string{"Acquire", "acquire", "Acquired", "acquired", "DoAcquire", "Gain", "Obtain"} {
			if b, ok := boolTrue(m[k]); ok && b {
				kind = EventAcquire
			}
		}
	}
	if kind == "" {
		for _, k := range []string{"End", "end", "Ended", "ended", "Release", "release", "Released", "Expire", "Expired", "Done"} {
			if b, ok := boolTrue(m[k]); ok && b {
				kind = EventEnd
			}
		}
	}
	// Key whose value is a bare credential string indicating the verb.
	if cred == "" {
		for _, k := range []string{"Acquire", "acquire", "Acquired", "Obtain", "Gain"} {
			if s := firstStringAny(m, []string{k}); s != "" {
				cred, kind = s, EventAcquire
			}
		}
		for _, k := range []string{"End", "end", "Ended", "Release", "Released", "Expire"} {
			if s := firstStringAny(m, []string{k}); s != "" {
				cred, kind = s, EventEnd
			}
		}
	}
	if kind == "" {
		// Scan every string value and every key for a verb hint.
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if kd := normalizeKind(k); kd != "" {
				kind = kd
				break
			}
			if s := rawString(m[k]); s != "" {
				if kd := normalizeKind(s); kd != "" {
					kind = kd
					break
				}
			}
		}
	}
	if cred == "" {
		cred = fallbackCredential(m)
	}
	if cred == "" {
		return nil
	}
	if kind == "" {
		kind = EventAcquire
	}
	ev := LeaseEvent{Frame: frame, HasFrame: hasFrame, Credential: cred, Kind: kind, raw: m}
	if v, ok := firstRaw(m, rootsKeys); ok {
		if ss, err := decodeFlexStrings(v); err == nil {
			ev.Roots = ss
		}
	}
	if v, ok := firstRaw(m, untilKeys); ok {
		if n, err := decodeFlexInt(v); err == nil {
			ev.Until, ev.HasUntil = n, true
		}
	}
	return []LeaseEvent{ev}
}

// fallbackCredential picks a non-verb string scalar under an unfamiliar key.
func fallbackCredential(m map[string]json.RawMessage) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Prefer key names that imply identity.
	for _, k := range keys {
		nk := normKey(k)
		if strings.Contains(nk, "cred") || strings.Contains(nk, "token") ||
			strings.Contains(nk, "lease") || strings.Contains(nk, "voucher") ||
			strings.Contains(nk, "ticket") || nk == "id" || strings.HasSuffix(nk, "id") {
			if s := rawString(m[k]); s != "" && normalizeKind(s) == "" {
				return s
			}
		}
	}
	for _, k := range keys {
		s := rawString(m[k])
		if s == "" || normalizeKind(s) != "" {
			continue
		}
		// Skip things that look like positions or sentences.
		if len(s) > 64 {
			continue
		}
		if _, err := decodeFlexInt(m[k]); err == nil {
			continue
		}
		return s
	}
	return ""
}

func boolTrue(raw json.RawMessage) (bool, bool) {
	if raw == nil {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	return false, false
}

func frameFromMap(m map[string]json.RawMessage, def int) int {
	if v, ok := firstRaw(m, frameKeys); ok {
		if n, err := decodeFlexInt(v); err == nil {
			return n
		}
	}
	return def
}

var acquireListKeys = []string{
	"Acquire", "acquire", "Acquires", "acquires", "Acquired", "acquired",
	"Gain", "Gained", "Obtain", "Obtained", "Take", "Grant", "Granted",
}
var endListKeys = []string{
	"End", "end", "Ends", "ends", "Ended", "ended", "Release", "releases",
	"Released", "released", "Expire", "Expired", "Finish", "Finished",
	"Complete", "Completed", "Cancel", "Cancelled", "Canceled",
}

func parseTokenListFrame(m map[string]json.RawMessage, frame int) []LeaseEvent {
	var out []LeaseEvent
	for _, k := range acquireListKeys {
		if raw, ok := m[k]; ok {
			if ss, err := decodeFlexStrings(raw); err == nil {
				for _, s := range ss {
					out = append(out, LeaseEvent{Frame: frame, HasFrame: true, Credential: s, Kind: EventAcquire})
				}
			}
		}
	}
	for _, k := range endListKeys {
		if raw, ok := m[k]; ok {
			if ss, err := decodeFlexStrings(raw); err == nil {
				for _, s := range ss {
					out = append(out, LeaseEvent{Frame: frame, HasFrame: true, Credential: s, Kind: EventEnd})
				}
			}
		}
	}
	return out
}

func tryParseBuckets(raw json.RawMessage) ([]EventBucket, bool) {
	raw = stripWS(raw)
	if len(raw) == 0 || raw[0] != '[' {
		// Maybe a map keyed by frame index.
		if raw != nil && len(raw) > 0 && raw[0] == '{' {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, false
			}
			out := make([]EventBucket, 0, len(m))
			for k, v := range m {
				idx, err := strconvAtoi(k)
				if err != nil {
					continue
				}
				b := EventBucket{Frame: idx}
				if evs, good := tryParseEventArray(v, idx); good {
					b.Events = evs
				} else {
					b.Events = parseEventItem(v, idx)
				}
				out = append(out, b)
			}
			return out, true
		}
		return nil, false
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, false
	}
	// Array of event arrays?
	if len(raws) > 0 && stripWS(raws[0])[0] == '[' {
		out := make([]EventBucket, 0, len(raws))
		for i, r := range raws {
			evs, _ := tryParseEventArray(r, i)
			out = append(out, EventBucket{Frame: i, Events: evs})
		}
		return out, true
	}
	// Array of bucket objects.
	var buckets []EventBucket
	if err := json.Unmarshal(raw, &buckets); err == nil && len(buckets) > 0 {
		for i := range buckets {
			if buckets[i].Frame == 0 {
				// Distinguish explicit frame 0 from unset: trust Index/Step keys.
				var mm map[string]json.RawMessage
				if json.Unmarshal(raws[i], &mm) == nil {
					buckets[i].Frame = frameFromMap(mm, i)
				}
			}
		}
		return buckets, true
	}
	return nil, false
}

func (b *EventBucket) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		// Maybe bare event array.
		var evs []LeaseEvent
		if err2 := json.Unmarshal(data, &evs); err2 != nil {
			return err
		}
		b.Events = evs
		return nil
	}
	b.Frame = frameFromMap(m, 0)
	if v, ok := firstRaw(m, flatEventKeys); ok {
		if evs, good := tryParseEventArray(v, b.Frame); good {
			b.Events = append(b.Events, evs...)
		}
	}
	if evs := parseTokenListFrame(m, b.Frame); len(evs) > 0 {
		b.Events = append(b.Events, evs...)
	}
	// Some shapes use "Items".
	if v, ok := m["Items"]; ok {
		if evs, good := tryParseEventArray(v, b.Frame); good {
			b.Events = append(b.Events, evs...)
		}
	}
	return nil
}

func strconvAtoi(s string) (int, error) {
	n := 0
	neg := false
	for i, r := range s {
		if i == 0 && r == '-' {
			neg = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, errBadInt
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

var errBadInt = &numError{}

type numError struct{}

func (*numError) Error() string { return "bad integer key" }
