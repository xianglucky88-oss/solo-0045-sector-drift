package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
)

var knownRequestKeys = map[string]bool{
	"sectors": true, "assets": true, "gates": true, "route": true, "policy": true,
	"transferbudget": true, "stagingbudget": true, "transferquota": true,
	"stagingcapacity": true, "bytesperframe": true, "quota": true, "quotas": true,
	"limits": true, "caps": true, "streamingpolicy": true, "slowdisk": true,
	"streaming": true, "transfer": true, "staging": true, "disk": true, "io": true,
}

func init() {
	for k := range transferAliases {
		knownRequestKeys[k] = true
	}
	for k := range stagingAliases {
		knownRequestKeys[k] = true
	}
}

func sdStrictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// UnmarshalJSON normalizes top-level snake/camel quota aliases while retaining
// strict unknown-field rejection for both request and nested documents.
func (r *Request) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	canonical := make(map[string]json.RawMessage, len(raw))
	var transferValue, stagingValue int64
	hasTransferValue, hasStagingValue := false, false

	for key, value := range raw {
		nk := sdNormalizeKey(key)
		switch nk {
		case "sectors":
			canonical["Sectors"] = value
		case "assets":
			canonical["Assets"] = value
		case "gates":
			canonical["Gates"] = value
		case "route":
			canonical["Route"] = value
		case "policy":
			canonical["Policy"] = value
		case "quota":
			canonical["Quota"] = value
		case "quotas":
			canonical["Quotas"] = value
		case "streaming":
			canonical["Streaming"] = value
		case "limits":
			canonical["Limits"] = value
		case "caps":
			canonical["Caps"] = value
		case "streamingpolicy":
			canonical["StreamingPolicy"] = value
		case "slowdisk":
			canonical["SlowDisk"] = value
		case "disk":
			canonical["Disk"] = value
		case "io":
			canonical["IO"] = value
		case "transfer":
			var n int64
			if err := json.Unmarshal(value, &n); err == nil {
				transferValue, hasTransferValue = n, true
			} else {
				q := new(Quota)
				if err := sdStrictUnmarshal(value, q); err != nil {
					return fmt.Errorf("invalid transfer quota: %w", err)
				}
				if tv, _ := quotaFrom(q); tv > 0 {
					transferValue, hasTransferValue = tv, true
				}
				canonical["Quota"] = value
			}
		case "staging":
			var n int64
			if err := json.Unmarshal(value, &n); err == nil {
				stagingValue, hasStagingValue = n, true
			} else {
				q := new(Quota)
				if err := sdStrictUnmarshal(value, q); err != nil {
					return fmt.Errorf("invalid staging quota: %w", err)
				}
				if _, sv := quotaFrom(q); sv > 0 {
					stagingValue, hasStagingValue = sv, true
				}
				canonical["Quota"] = value
			}
		default:
			if c, ok := sdQuotaAliasKind(nk); ok {
				var n int64
				if err := json.Unmarshal(value, &n); err == nil {
					if c == "TransferBudget" {
						transferValue, hasTransferValue = n, true
					} else {
						stagingValue, hasStagingValue = n, true
					}
					continue
				}
				// Named nested quota objects are handled by their canonical
				// fields above; reject other non-scalar aliases strictly.
				return fmt.Errorf("unsupported quota field %q", key)
			}
			if !knownRequestKeys[nk] {
				return fmt.Errorf("unknown request field %q", key)
			}
			canonical[key] = value
		}
	}
	if hasTransferValue {
		b, _ := json.Marshal(transferValue)
		canonical["TransferBudget"] = b
	}
	if hasStagingValue {
		b, _ := json.Marshal(stagingValue)
		canonical["StagingBudget"] = b
	}

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return err
	}
	type plain Request
	var pp plain
	if err := sdStrictUnmarshal(encoded, &pp); err != nil {
		return err
	}
	*r = Request(pp)
	return nil
}
