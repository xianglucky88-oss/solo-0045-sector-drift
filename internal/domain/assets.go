package domain

import "fmt"

func indexAssets(src []Asset) (map[string]Asset, error) {
	out := make(map[string]Asset, len(src))
	for _, a := range src {
		if _, ok := out[a.ID]; ok || !label.MatchString(a.ID) || a.Bytes < 1 || a.Bytes > 1<<32 || len(a.After) > 64 {
			return nil, fmt.Errorf("invalid asset %q", a.ID)
		}
		out[a.ID] = a
	}
	for _, a := range src {
		if err := knownAssets(a.After, out); err != nil {
			return nil, fmt.Errorf("asset %s: %w", a.ID, err)
		}
	}
	return out, nil
}
func knownAssets(ids []string, assets map[string]Asset) error {
	seen := make(Set)
	for _, id := range ids {
		if _, ok := assets[id]; !ok || seen[id] {
			return fmt.Errorf("unknown or repeated asset %q", id)
		}
		seen[id] = true
	}
	return nil
}
