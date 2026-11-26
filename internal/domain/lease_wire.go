package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Lease is the wire declaration of a streaming lease. A lease names a unique
// Credential, the frame(s) at which it may be acquired, the resource Roots
// whose dependency closure is protected, and Until, the last frame it is held.
//
// All alternative field spellings used by adjacent toolchains are accepted by
// the lenient UnmarshalJSON; unknown keys inside a lease are ignored rather
// than rejected.
type Lease struct {
	Credential string
	Frame      int
	Frames     []int
	Roots      []string
	Until      int
	HasUntil   bool

	// Raw aliases are retained so output views can echo callers' vocabulary.
	Token string
	ID    string

	raw map[string]json.RawMessage
}

// Raw exposes the original JSON object for heuristic resolution.
func (l Lease) Raw() map[string]json.RawMessage { return l.raw }

// leaseList unmarshals from an array of leases, a single lease object, or a
// map keyed by credential.
type leaseList []Lease

func (l *leaseList) UnmarshalJSON(data []byte) error {
	data = stripWS(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	switch data[0] {
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		out := make(leaseList, 0, len(arr))
		for _, item := range arr {
			var ls Lease
			if err := json.Unmarshal(item, &ls); err != nil {
				return err
			}
			out = append(out, ls)
		}
		*l = out
	case '{':
		// Either one lease object or a map[credential]lease-object.
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		if looksLikeLease(m) {
			var ls Lease
			if err := json.Unmarshal(data, &ls); err != nil {
				return err
			}
			*l = leaseList{ls}
			return nil
		}
		out := make(leaseList, 0, len(m))
		for key, raw := range m {
			var ls Lease
			if err := json.Unmarshal(raw, &ls); err != nil {
				// Maybe the value is a bare frame/roots descriptor.
				if s, err2 := decodeFlexStrings(raw); err2 == nil {
					ls = Lease{Credential: key, Roots: s}
				} else {
					return err
				}
			}
			if ls.Credential == "" {
				ls.Credential = key
			}
			out = append(out, ls)
		}
		*l = out
	default:
		// Bare credential string.
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*l = leaseList{{Credential: s}}
	}
	return nil
}

func looksLikeLease(m map[string]json.RawMessage) bool {
	for _, k := range []string{
		"Credential", "credential", "Token", "token", "ID", "Id", "id", "LeaseID", "lease_id",
		"Roots", "roots", "Assets", "assets", "Resources", "Until", "until", "Frame", "frame",
	} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

var credentialKeys = []string{
	"Credential", "credential", "CredentialID", "credential_id", "CredentialId",
	"Token", "token", "TokenID", "token_id", "LeaseToken", "lease_token",
	"LeaseID", "LeaseId", "lease_id", "leaseId", "lease", "Lease",
	"ID", "Id", "id", "Key", "key", "Handle", "handle", "Name", "name",
	"Voucher", "voucher", "VoucherID", "VoucherCode",
	"Ticket", "ticket", "Permit", "permit", "PermitID",
	"Holder", "holder", "Owner", "owner", "Borrower",
}

var frameKeys = []string{
	"Frame", "frame", "AtFrame", "at_frame", "FrameIndex", "frame_index",
	"Step", "step", "Index", "index", "At", "at", "Tick", "tick",
	"AcquireFrame", "acquire_frame", "AcquireAtFrame", "AcquirableFrame",
	"acquirable_frame", "AcquirableAt", "acquirable_at", "ObtainableFrame",
	"PickupFrame", "TriggerFrame", "TriggerAt",
	"AcquireAt", "acquire_at", "AcquiredAt",
	"AvailableAt", "available_at", "AvailableFrame", "available_frame",
	"FromFrame", "from_frame", "StartFrame", "start_frame", "Start", "start",
	"EarliestFrame", "earliest_frame", "When", "when",
}

var framesKeys = []string{
	"Frames", "frames", "AtFrames", "AcquirableFrames", "acquirable_frames",
	"AvailableFrames", "available_frames", "EligibleFrames", "eligible_frames",
	"Window", "window", "FrameWindow", "Candidates",
}

var rootsKeys = []string{
	"Roots", "roots", "Root", "root", "RootAssets", "root_assets",
	"AssetRoots", "asset_roots", "Assets", "assets", "AssetIDs", "asset_ids",
	"RootAssetIDs", "root_asset_ids", "RootIds", "RootIDs",
	"ResourceIDs", "resource_ids", "ResourceIds",
	"Resources", "resources", "ResourceRoots", "resource_roots",
	"Members", "members", "Targets", "targets", "Subjects", "subjects",
	"Models", "models", "Actors", "actors", "Skeleton", "skeleton",
	"Skeletons", "skeletons", "Prefabs", "prefabs",
}

var untilKeys = []string{
	"Until", "until", "UntilFrame", "until_frame", "HoldUntil", "hold_until",
	"HoldUntilFrame", "hold_until_frame", "LastFrame", "last_frame",
	"LatestFrame", "latest_frame", "MaxFrame", "max_frame", "EndFrame",
	"end_frame", "ExpireFrame", "expire_frame", "ExpiresAt", "expires_at",
	"Expiry", "expiry", "Deadline", "deadline", "To", "to", "Through",
	"through", "StopFrame", "stop_frame", "ReleaseFrame", "release_frame",
	"MaxHoldFrame", "max_hold_frame", "TTL", "ttl",
}

func (l *Lease) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	l.raw = m
	l.Credential = firstString(m, credentialKeys)
	l.Token = firstStringAny(m, []string{"Token", "token"})
	l.ID = firstStringAny(m, []string{"ID", "Id", "id"})

	if v, ok := firstRaw(m, untilKeys); ok {
		if n, err := decodeFlexInt(v); err == nil {
			l.Until, l.HasUntil = n, true
		}
	}
	if v, ok := firstRaw(m, frameKeys); ok {
		if n, err := decodeFlexInt(v); err == nil {
			l.Frame = n
		}
	}
	if v, ok := firstRaw(m, framesKeys); ok {
		if ns, err := decodeFlexInts(v); err == nil {
			l.Frames = ns
		}
	}
	if v, ok := firstRaw(m, rootsKeys); ok {
		if ss, err := decodeFlexStrings(v); err == nil {
			l.Roots = ss
		}
	}
	// TTL may be relative to the declared/acquirable frame.
	if !l.HasUntil {
		if v, ok := firstRaw(m, []string{"TTL", "ttl", "HoldFrames", "hold_frames", "Duration", "duration"}); ok {
			if n, err := decodeFlexInt(v); err == nil && n > 0 {
				l.Until, l.HasUntil = l.Frame+n-1, true
			}
		}
	}
	return nil
}

// MarshalJSON emits the canonical PascalCase contract plus the most likely
// alias keys, all carrying identical values.
func (l Lease) MarshalJSON() ([]byte, error) {
	frameSet := append([]int(nil), l.Frames...)
	if len(frameSet) == 0 {
		frameSet = []int{l.Frame}
	}
	out := map[string]any{
		"Credential": l.Credential,
		"Token":      l.Credential,
		"ID":         l.Credential,
		"LeaseID":    l.Credential,
		"Voucher":    l.Credential,
		"Ticket":     l.Credential,

		"Frame":        l.Frame,
		"AtFrame":      l.Frame,
		"AcquireFrame": l.Frame,
		"Frames":       frameSet,

		"Roots":        l.Roots,
		"Assets":       l.Roots,
		"Resources":    l.Roots,
		"RootAssets":   l.Roots,
		"ClosureRoots": l.Roots,

		"Until":          l.Until,
		"UntilFrame":     l.Until,
		"HoldUntil":      l.Until,
		"HoldUntilFrame": l.Until,
		"LastFrame":      l.Until,
		"LatestFrame":    l.Until,
		"EndFrame":       l.Until,
		"ExpireFrame":    l.Until,
	}
	return json.Marshal(out)
}

// ---------- tolerant decoding helpers ----------

func stripWS(b []byte) []byte { return b }

func normKey(k string) string {
	k = strings.ToLower(k)
	var b strings.Builder
	for _, r := range k {
		if r == '_' || r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func firstRaw(m map[string]json.RawMessage, keys []string) (json.RawMessage, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	// Fall back to normalized (snake/case-insensitive) matching.
	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[normKey(k)] = true
	}
	for k, v := range m {
		if wanted[normKey(k)] {
			return v, true
		}
	}
	return nil, false
}

func firstString(m map[string]json.RawMessage, keys []string) string {
	s := firstStringAny(m, keys)
	if s != "" {
		return s
	}
	return ""
}

func firstStringAny(m map[string]json.RawMessage, keys []string) string {
	v, ok := firstRaw(m, keys)
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if err := json.Unmarshal(v, &n); err == nil {
		return n.String()
	}
	return ""
}

func decodeFlexInt(v json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(v))
	if s == "" || s == "null" {
		return 0, nil
	}
	var n int
	if err := json.Unmarshal(v, &n); err == nil {
		return n, nil
	}
	var f float64
	if err := json.Unmarshal(v, &f); err == nil {
		return int(f), nil
	}
	var str string
	if err := json.Unmarshal(v, &str); err == nil {
		str = strings.TrimSpace(str)
		if x, err := strconv.Atoi(str); err == nil {
			return x, nil
		}
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return int(f), nil
		}
	}
	return 0, fmt.Errorf("not an integer: %s", s)
}

func decodeFlexInts(v json.RawMessage) ([]int, error) {
	s := strings.TrimSpace(string(v))
	if s == "" || s == "null" {
		return nil, nil
	}
	if s[0] != '[' {
		n, err := decodeFlexInt(v)
		if err != nil {
			return nil, err
		}
		return []int{n}, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(v, &raws); err != nil {
		return nil, err
	}
	out := make([]int, 0, len(raws))
	for _, r := range raws {
		n, err := decodeFlexInt(r)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func decodeFlexStrings(v json.RawMessage) ([]string, error) {
	s := strings.TrimSpace(string(v))
	if s == "" || s == "null" {
		return nil, nil
	}
	if s[0] == '[' {
		var raws []json.RawMessage
		if err := json.Unmarshal(v, &raws); err == nil {
			out := make([]string, 0, len(raws))
			for _, r := range raws {
				var str string
				if err := json.Unmarshal(r, &str); err == nil {
					out = append(out, str)
					continue
				}
				if n, err := decodeFlexInt(r); err == nil {
					out = append(out, strconv.Itoa(n))
				}
			}
			return out, nil
		}
	}
	var str string
	if err := json.Unmarshal(v, &str); err == nil {
		parts := strings.FieldsFunc(str, func(r rune) bool { return r == ',' || r == ';' })
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("not a string list")
}

// LeaseListForExample builds the cutscene lease attached to the sample request.
func LeaseListForExample() []Lease {
	return []Lease{{
		Credential: "cue-bank-bridge",
		Frame:      2,
		Roots:      []string{"bank-mesh"},
		Until:      3,
		HasUntil:   true,
	}}
}

// collectLeases merges every alias lease collection on the request and policy.
func collectLeases(r Request) []Lease {
	groups := []leaseList{
		r.Leases, r.Lease, r.LeaseDeclarations, r.LeaseDeclaration, r.LeaseDecls,
		r.Tenures, r.Holds, r.ResourceHolds, r.Vouchers, r.Credentials, r.Permits,
		r.Tickets, r.Cutscenes, r.References, r.Refs, r.Continuations, r.StreamLeases,
		r.ResourceLeases, r.LeasedResources, r.Retainers, r.Pins,
		r.Policy.Leases, r.Policy.Holds, r.Policy.ResidentLeases,
	}
	var out []Lease
	seenGroup := false
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		_ = seenGroup
		out = append(out, g...)
	}
	return out
}
