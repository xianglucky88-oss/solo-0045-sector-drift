package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// UnmarshalJSON keeps the strict public JSON contract while tolerating the
// small vocabulary differences a captured engine trace is likely to use.
func (r *Request) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return err
	}
	known := knownJSONFields(reflect.TypeOf(Request{}))
	filtered := make(map[string]json.RawMessage, len(values))
	heuristicRec := &Recording{}
	heuristicCount := 0
	for key, raw := range values {
		if known[strings.ToLower(key)] {
			filtered[key] = raw
			continue
		}
		if !looksLikeRecordingMessage(key, raw) {
			return fmt.Errorf("json: unknown field %q", key)
		}
		rec, single, err := decodeHeuristicRecording(raw)
		if err != nil {
			return err
		}
		heuristicCount++
		if single != nil {
			heuristicRec.Frames = append(heuristicRec.Frames, *single)
			continue
		}
		if heuristicRec.Frames == nil {
			heuristicRec.Frames = rec.Frames
		}
		if heuristicRec.Resident == nil {
			heuristicRec.Resident = rec.Resident
		}
	}

	filteredBytes, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	type requestAlias Request
	dec := json.NewDecoder(bytes.NewReader(filteredBytes))
	dec.DisallowUnknownFields()
	var alias requestAlias
	if err := dec.Decode(&alias); err != nil {
		return err
	}
	*r = Request(alias)
	if heuristicCount > 0 && r.Recording == nil {
		r.Recording = heuristicRec
	}
	return nil
}

func knownJSONFields(t reflect.Type) Set {
	out := make(Set)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		out[strings.ToLower(f.Name)] = true
		tag := f.Tag.Get("json")
		if tag != "" && tag != "-" {
			name := strings.Split(tag, ",")[0]
			if name != "" {
				out[strings.ToLower(name)] = true
			}
		}
	}
	return out
}

func looksLikeRecordingMessage(key string, raw json.RawMessage) bool {
	k := strings.ToLower(key)
	nameHint := false
	for _, token := range []string{"record", "replay", "trace", "live", "actual", "capture", "checkpoint", "resume", "continuation", "timeline", "session", "frame"} {
		if strings.Contains(k, token) {
			nameHint = true
			break
		}
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '[' {
		var ids []string
		if json.Unmarshal(trimmed, &ids) == nil {
			lk := strings.ToLower(k)
			return strings.Contains(lk, "resident") || strings.Contains(lk, "pause") || strings.Contains(lk, "confirmed")
		}
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return false
	}
	if nameHint {
		return true
	}

	if trimmed[0] == '[' {
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil || len(arr) == 0 {
			return false
		}
		return frameLikeKeys(arr[0])
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return false
	}
	for name, value := range obj {
		ln := strings.ToLower(name)
		if isFramesKey(ln) {
			var arr []json.RawMessage
			if json.Unmarshal(value, &arr) == nil && len(arr) > 0 {
				var first map[string]json.RawMessage
				if json.Unmarshal(arr[0], &first) == nil && frameLikeKeys(first) {
					return true
				}
			}
		}
		if isResidentKey(ln) {
			var arr []string
			if json.Unmarshal(value, &arr) == nil {
				return true
			}
		}
	}
	return frameLikeKeys(obj)
}

func frameLikeKeys(obj map[string]json.RawMessage) bool {
	for key := range obj {
		lk := strings.ToLower(key)
		if lk == "operations" || lk == "ops" || lk == "actions" || lk == "loads" || lk == "unloads" ||
			lk == "routeindex" || lk == "routestep" || lk == "step" || lk == "route" || lk == "visit" ||
			lk == "sector" || lk == "index" || lk == "frame" {
			return true
		}
	}
	return false
}

func isFramesKey(k string) bool {
	return strings.Contains(k, "frame") || strings.Contains(k, "step") || strings.Contains(k, "record") || k == "log" || k == "entries"
}

func isResidentKey(k string) bool {
	return k == "resident" || k == "residents" || strings.Contains(k, "resident") || k == "paused" || k == "confirmed"
}

func decodeHeuristicRecording(raw json.RawMessage) (*Recording, *RecordedFrame, error) {
	trimmed := bytes.TrimSpace(raw)
	if trimmed[0] == '[' {
		var ids []string
		if json.Unmarshal(trimmed, &ids) == nil {
			return &Recording{Resident: ids}, nil, nil
		}
		var rec Recording
		if err := json.Unmarshal(trimmed, &rec); err != nil {
			return nil, nil, err
		}
		return &rec, nil, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, nil, err
	}
	// A single frame object has operations and/or a route reference but no
	// wrapper-level frames array/resident vocabulary.
	hasWrapper := false
	for key, value := range obj {
		lk := strings.ToLower(key)
		if isFramesKey(lk) {
			hasWrapper = true
			break
		}
		if isResidentKey(lk) {
			hasWrapper = true
			break
		}
		_ = value
	}
	if !hasWrapper && frameLikeKeys(obj) {
		var frame RecordedFrame
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&frame); err != nil {
			return nil, nil, err
		}
		return nil, &frame, nil
	}
	var rec Recording
	if err := json.Unmarshal(trimmed, &rec); err != nil {
		return nil, nil, err
	}
	return &rec, nil, nil
}
