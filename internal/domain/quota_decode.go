package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

func sdNormalizeKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var transferAliases = map[string]string{
	"transferbudget":        "TransferBudget",
	"transferquota":         "TransferBudget",
	"transferperframe":      "TransferBudget",
	"transferbytesperframe": "TransferBudget",
	"bytestransferperframe": "TransferBudget",
	"bytesperframe":         "TransferBudget",
	"perframetransfer":      "TransferBudget",
	"perframebytes":         "TransferBudget",
	"frametransferbudget":   "TransferBudget",
	"frametransferbytes":    "TransferBudget",
	"maxtransferbytes":      "TransferBudget",
	"transferlimit":         "TransferBudget",
	"transferallowance":     "TransferBudget",
	"transferrate":          "TransferBudget",
	"downloadbudget":        "TransferBudget",
	"downloadquota":         "TransferBudget",
	"downloadbytesperframe": "TransferBudget",
	"downloadlimit":         "TransferBudget",
	"maxdownloadbytes":      "TransferBudget",
	"bandwidth":             "TransferBudget",
	"rate":                  "TransferBudget",
	"throughput":            "TransferBudget",
	"iobudget":              "TransferBudget",
	"diskbudget":            "TransferBudget",
	"diskquota":             "TransferBudget",
	"slowdiskbudget":        "TransferBudget",
}

var stagingAliases = map[string]string{
	"stagingbudget":   "StagingBudget",
	"stagingcapacity": "StagingBudget",
	"staginglimit":    "StagingBudget",
	"stagingbytes":    "StagingBudget",
	"stagingquota":    "StagingBudget",
	"stagingsize":     "StagingBudget",
	"stagingspace":    "StagingBudget",
	"stagingarea":     "StagingBudget",
	"stagebudget":     "StagingBudget",
	"stagecapacity":   "StagingBudget",
	"stagelimit":      "StagingBudget",
	"stagebytes":      "StagingBudget",
	"maxstagingbytes": "StagingBudget",
	"scratchbudget":   "StagingBudget",
	"scratchcapacity": "StagingBudget",
	"scratchbytes":    "StagingBudget",
	"pendingbudget":   "StagingBudget",
	"pendingcapacity": "StagingBudget",
}

var knownPolicyKeys = map[string]bool{
	"budget": true, "lookahead": true, "retain": true, "pinned": true,
	"quota": true, "quotas": true, "streaming": true, "slowdisk": true,
	"transfer": true, "staging": true, "disk": true, "io": true, "cache": true,
}

func init() {
	for k := range transferAliases {
		knownPolicyKeys[k] = true
	}
	for k := range stagingAliases {
		knownPolicyKeys[k] = true
	}
}

func sdQuotaAliasKind(nk string) (string, bool) {
	if c, ok := transferAliases[nk]; ok {
		return c, true
	}
	if c, ok := stagingAliases[nk]; ok {
		return c, true
	}
	if strings.Contains(nk, "transfer") || strings.Contains(nk, "download") {
		return "TransferBudget", true
	}
	if strings.Contains(nk, "staging") || strings.HasPrefix(nk, "stage") || strings.Contains(nk, "scratch") {
		return "StagingBudget", true
	}
	return "", false
}

// UnmarshalJSON accepts the canonical PascalCase policy document and the
// common snake/camel spellings for the two streaming limits. It retains
// unknown-field rejection for the policy and nested quota objects.
func (p *Policy) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	canonical := make(map[string]json.RawMessage, len(raw))
	var transferValue, stagingValue int64
	hasTransferValue, hasStagingValue := false, false

	nested := map[string]string{
		"quota": "Quota", "quotas": "Quotas", "streaming": "Streaming",
		"limits": "Limits", "caps": "Caps", "slowdisk": "SlowDisk",
		"disk": "Disk", "io": "IO", "cache": "Cache",
	}

	for key, value := range raw {
		nk := sdNormalizeKey(key)

		switch nk {
		case "budget":
			canonical["Budget"] = value
			continue
		case "lookahead":
			canonical["Lookahead"] = value
			continue
		case "retain":
			canonical["Retain"] = value
			continue
		case "pinned":
			canonical["Pinned"] = value
			continue
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
			}
			continue
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
			}
			continue
		}

		if name, ok := nested[nk]; ok {
			canonical[name] = value
			continue
		}

		if c, ok := sdQuotaAliasKind(nk); ok {
			var n int64
			if err := json.Unmarshal(value, &n); err == nil {
				canonical[c] = value
				if c == "TransferBudget" {
					transferValue, hasTransferValue = n, true
				} else {
					stagingValue, hasStagingValue = n, true
				}
				continue
			}
			return fmt.Errorf("unsupported policy quota field %q", key)
		}

		if !knownPolicyKeys[nk] {
			return fmt.Errorf("unknown policy field %q", key)
		}
		canonical[key] = value
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
	type plain Policy
	var pp plain
	if err := sdStrictUnmarshal(encoded, &pp); err != nil {
		return err
	}
	*p = Policy(pp)
	return nil
}
