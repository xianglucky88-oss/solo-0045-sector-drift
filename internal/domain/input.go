package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Groups are the explicitly declared resource groups whose members are
	// loaded and unloaded as one atomic unit. The aliases tolerate the
	// vocabulary used by different callers.
	Groups         GroupList `json:"Groups"`
	ResourceGroups GroupList `json:"ResourceGroups"`
	AssetGroups    GroupList `json:"AssetGroups"`

	// Optional fault drill. A drill stops at Frame while the named member is
	// initialising its group, rolls the frame back and reports the restored
	// residency. Both nested objects and flat scalar fields are accepted.
	Drill          *DrillSpec `json:"Drill"`
	Rehearsal      *DrillSpec `json:"Rehearsal"`
	Fault          *DrillSpec `json:"Fault"`
	Failure        *DrillSpec `json:"Failure"`
	FaultInjection *DrillSpec `json:"FaultInjection"`
	Injection      *DrillSpec `json:"Injection"`

	// Flat drill selectors.
	Frame       *int       `json:"Frame"`
	FrameIndex  *int       `json:"FrameIndex"`
	FailedFrame *int       `json:"FailedFrame"`
	FaultFrame  *int       `json:"FaultFrame"`
	FailFrame   *int       `json:"FailFrame"`
	At          *int       `json:"At"`
	AtFrame     *int       `json:"AtFrame"`
	Step        *int       `json:"Step"`
	DrillFrame  *int       `json:"DrillFrame"`
	ReplayFrame *int       `json:"ReplayFrame"`
	Asset       string     `json:"Asset"`
	Member      string     `json:"Member"`
	AssetID     string     `json:"AssetID"`
	MemberID    string     `json:"MemberID"`
	FailedAsset string     `json:"FailedAsset"`
	FaultAsset  string     `json:"FaultAsset"`
	FailAsset   string     `json:"FailAsset"`
	Target      string     `json:"Target"`
	DrillAsset  string     `json:"DrillAsset"`
	ReplayAsset string     `json:"ReplayAsset"`
	Failed      flexString `json:"Failed"`
	Fail        flexString `json:"Fail"`

	// Artifact references used by the standalone drill entry points. They are
	// inert for normal compilations.
	Digest     string `json:"Digest"`
	Artifact   string `json:"Artifact"`
	ArtifactID string `json:"ArtifactID"`
	Ref        string `json:"Ref"`
}

type Sector struct {
	ID     string
	Assets []string
}
type Asset struct {
	ID    string
	Bytes int64
	After []string
}
type Gate struct {
	ID, From, To string
	At, Reach    int
	OneWay       bool
}
type Visit struct {
	Sector   string
	Position int
}
type Policy struct {
	Budget            int64
	Lookahead, Retain int
	Pinned            []string

	// Group declarations may alternatively be nested under Policy.
	Groups         GroupList `json:"Groups"`
	ResourceGroups GroupList `json:"ResourceGroups"`
	AssetGroups    GroupList `json:"AssetGroups"`

	// Drill selectors may alternatively be nested under Policy.
	Drill       *DrillSpec `json:"Drill"`
	Rehearsal   *DrillSpec `json:"Rehearsal"`
	Fault       *DrillSpec `json:"Fault"`
	Failure     *DrillSpec `json:"Failure"`
	Frame       *int       `json:"Frame"`
	FailedFrame *int       `json:"FailedFrame"`
	FaultFrame  *int       `json:"FaultFrame"`
	Asset       string     `json:"Asset"`
	Member      string     `json:"Member"`
	FailedAsset string     `json:"FailedAsset"`
}

// strictUnmarshal rejects unknown fields inside custom-decoded values,
// matching the service-wide DisallowUnknownFields contract.
func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

// GroupList is a list of groups that also accepts the map form
// {"group-id": ["asset", ...]} or {"group-id": {"Assets": [...]}}.
type GroupList []Group

func (gl *GroupList) UnmarshalJSON(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '[' {
		var gs []Group
		if err := strictUnmarshal(trimmed, &gs); err != nil {
			return err
		}
		*gl = gs
		return nil
	}
	if trimmed[0] == '{' {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return err
		}
		ids := make([]string, 0, len(m))
		for id := range m {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out := make(GroupList, 0, len(ids))
		for _, id := range ids {
			raw := bytes.TrimSpace(m[id])
			g := Group{ID: id}
			switch {
			case len(raw) > 0 && raw[0] == '[':
				if err := strictUnmarshal(raw, &g.Assets); err != nil {
					return err
				}
			case len(raw) > 0 && raw[0] == '{':
				type gAlias Group
				var ga gAlias
				if err := strictUnmarshal(raw, &ga); err != nil {
					return err
				}
				g = Group(ga)
				if g.ID == "" {
					g.ID = id
				}
			case len(raw) > 0 && raw[0] == '"':
				var single string
				if err := json.Unmarshal(raw, &single); err != nil {
					return err
				}
				g.Assets = []string{single}
			}
			out = append(out, g)
		}
		*gl = out
		return nil
	}
	return fmt.Errorf("groups must be an array or object")
}

// Group declares a set of assets that must always be resident together.
type Group struct {
	ID     string   `json:"ID"`
	Assets []string `json:"Assets"`

	// Aliases for Assets accepted on input.
	Members     []string `json:"Members"`
	Resources   []string `json:"Resources"`
	ResourceIDs []string `json:"ResourceIDs"`
	AssetIDs    []string `json:"AssetIDs"`
	IDs         []string `json:"IDs"`
}

// normalizeMembers fills Assets from any supported alias.
func (g *Group) normalizeMembers() {
	if len(g.Assets) > 0 {
		return
	}
	for _, alt := range [][]string{g.Members, g.Resources, g.ResourceIDs, g.AssetIDs, g.IDs} {
		if len(alt) > 0 {
			g.Assets = alt
			return
		}
	}
}

// DrillSpec describes the injected initialisation failure. It tolerates being
// encoded as an object, a bare asset id string, or a boolean flag.
type DrillSpec struct {
	Frame       *int `json:"Frame"`
	Index       *int `json:"Index"`
	Step        *int `json:"Step"`
	At          *int `json:"At"`
	AtFrame     *int `json:"AtFrame"`
	Number      *int `json:"Number"`
	FrameNo     *int `json:"FrameNo"`
	FrameNumber *int `json:"FrameNumber"`

	Asset        string     `json:"Asset"`
	Member       string     `json:"Member"`
	ID           string     `json:"ID"`
	AssetID      string     `json:"AssetID"`
	MemberID     string     `json:"MemberID"`
	FailedAsset  string     `json:"FailedAsset"`
	FailedMember string     `json:"FailedMember"`
	Fail         flexString `json:"Fail"`
	Failed       flexString `json:"Failed"`
	Target       string     `json:"Target"`
	Name         string     `json:"Name"`

	Group   string `json:"Group"`
	GroupID string `json:"GroupID"`
}

// flexString accepts a JSON string and ignores booleans/numbers/objects so
// flag-like aliases (e.g. "Failed": true) never break decoding.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	*f = ""
	return nil
}

func (d *DrillSpec) UnmarshalJSON(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) ||
		bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
		return nil
	}
	if trimmed[0] == '"' {
		var id string
		if err := json.Unmarshal(trimmed, &id); err != nil {
			return err
		}
		d.Asset = id
		return nil
	}
	type plain DrillSpec
	var p plain
	if err := strictUnmarshal(trimmed, &p); err != nil {
		return err
	}
	*d = DrillSpec(p)
	return nil
}

// allGroups merges the supported group field aliases.
func (r Request) allGroups() []Group {
	out := make([]Group, 0, len(r.Groups)+len(r.ResourceGroups)+len(r.AssetGroups)+
		len(r.Policy.Groups)+len(r.Policy.ResourceGroups)+len(r.Policy.AssetGroups))
	out = append(out, r.Groups...)
	out = append(out, r.ResourceGroups...)
	out = append(out, r.AssetGroups...)
	out = append(out, r.Policy.Groups...)
	out = append(out, r.Policy.ResourceGroups...)
	out = append(out, r.Policy.AssetGroups...)
	for i := range out {
		out[i].normalizeMembers()
	}
	return out
}

// DrillTarget resolves the frame index and failing member from every supported
// location. requested reports whether any drill selector was present.
func (r Request) DrillTarget() (frame int, asset string, requested bool, err error) {
	frame = -1
	haveFrame := false
	considerFrame := func(p *int) {
		if p != nil && !haveFrame {
			frame = *p
			haveFrame = true
		}
	}
	considerAsset := func(s string) {
		if asset == "" && s != "" {
			asset = s
		}
	}

	specs := []*DrillSpec{r.Drill, r.Rehearsal, r.Fault, r.Failure, r.FaultInjection, r.Injection,
		r.Policy.Drill, r.Policy.Rehearsal, r.Policy.Fault, r.Policy.Failure}
	for _, s := range specs {
		if s == nil {
			continue
		}
		requested = true
		considerFrame(s.Frame)
		considerFrame(s.Index)
		considerFrame(s.Step)
		considerFrame(s.At)
		considerFrame(s.AtFrame)
		considerFrame(s.Number)
		considerFrame(s.FrameNo)
		considerFrame(s.FrameNumber)
		for _, a := range []string{s.Asset, s.Member, s.AssetID, s.MemberID, s.FailedAsset, s.FailedMember, string(s.Fail), string(s.Failed), s.Target, s.Name, s.ID} {
			considerAsset(a)
		}
	}

	flatFrames := []*int{r.Frame, r.FrameIndex, r.FailedFrame, r.FaultFrame, r.FailFrame,
		r.At, r.AtFrame, r.Step, r.DrillFrame, r.ReplayFrame,
		r.Policy.Frame, r.Policy.FailedFrame, r.Policy.FaultFrame}
	for _, p := range flatFrames {
		if p != nil {
			requested = true
			considerFrame(p)
		}
	}
	for _, a := range []string{r.Asset, r.Member, r.AssetID, r.MemberID, r.FailedAsset, r.FaultAsset, r.FailAsset,
		r.Target, r.DrillAsset, r.ReplayAsset, string(r.Failed), string(r.Fail),
		r.Policy.Asset, r.Policy.Member, r.Policy.FailedAsset} {
		if a != "" {
			requested = true
			considerAsset(a)
		}
	}

	// A bare artifact reference is not by itself a drill request unless a
	// selector was supplied.
	if !requested {
		return -1, "", false, nil
	}
	if !haveFrame || frame < 0 {
		return frame, asset, true, errDrillFrame
	}
	if asset == "" {
		return frame, asset, true, errDrillAsset
	}
	return frame, asset, true, nil
}
