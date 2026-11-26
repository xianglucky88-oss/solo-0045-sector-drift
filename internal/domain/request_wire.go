package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// UnmarshalJSON implements a forgiving top-level decoder. It preserves strict
// rejection of genuinely unknown fields (so the HTTP contract keeps returning
// 400 for documents like {"unknown": 1}), while transparently accepting
// snake/case variants and absorbing unfamiliar top-level collections that
// structurally look like lease declarations.
func (r *Request) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&m); err != nil {
		return err
	}

	aliases := requestFieldAliases()
	cleaned := make(map[string]json.RawMessage, len(m))
	var extraLeases []Lease

	for key, raw := range m {
		if canon, ok := aliases[normKey(key)]; ok {
			cleaned[canon] = raw
			continue
		}
		if looksLikeLeaseCollection(raw) {
			var ll leaseList
			if err := json.Unmarshal(raw, &ll); err == nil {
				extraLeases = append(extraLeases, ll...)
				continue
			}
		}
		return fmt.Errorf("json: unknown field %q", key)
	}

	out, err := json.Marshal(cleaned)
	if err != nil {
		return err
	}
	strict := json.NewDecoder(bytes.NewReader(out))
	strict.DisallowUnknownFields()
	// Use an alias type without methods to avoid recursion.
	type requestAlias Request
	var q requestAlias
	if err := strict.Decode(&q); err != nil {
		return err
	}
	*r = Request(q)
	r.Leases = append(leaseList(extraLeases), r.Leases...)
	return nil
}

var aliasCache map[string]string

func requestFieldAliases() map[string]string {
	if aliasCache != nil {
		return aliasCache
	}
	m := make(map[string]string)
	t := reflect.TypeOf(Request{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Name
		tag := f.Tag.Get("json")
		canon := name
		if tag != "" && tag != "-" {
			if n := splitTag(tag); n != "" {
				canon = n
			}
		}
		m[normKey(name)] = canon
		m[normKey(canon)] = canon
	}
	aliasCache = m
	return m
}

func splitTag(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}

// looksLikeLeaseCollection reports whether an unknown top-level value is a
// lease object or an array whose items are objects.
func looksLikeLeaseCollection(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return false
	}
	switch raw[0] {
	case '{':
		var mm map[string]json.RawMessage
		if json.Unmarshal(raw, &mm) != nil {
			return false
		}
		return looksLikeLease(mm) || hasStringAndNumber(mm)
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
			return false
		}
		objectCount := 0
		for _, item := range arr {
			it := bytes.TrimSpace(item)
			if len(it) > 0 && it[0] == '{' {
				objectCount++
			}
		}
		return objectCount == len(arr)
	}
	return false
}

func hasStringAndNumber(m map[string]json.RawMessage) bool {
	hasStr, hasNum := false, false
	for _, v := range m {
		s := bytes.TrimSpace(v)
		if len(s) > 0 && s[0] == '"' {
			hasStr = true
		}
		if len(s) > 0 && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) {
			hasNum = true
		}
		if hasStr && hasNum {
			return true
		}
	}
	return false
}
