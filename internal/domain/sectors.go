package domain

import "fmt"

func indexSectors(src []Sector, assets map[string]Asset) (map[string]Sector, error) {
	out := make(map[string]Sector, len(src))
	for _, s := range src {
		if _, ok := out[s.ID]; ok || !label.MatchString(s.ID) || len(s.Assets) == 0 || len(s.Assets) > 128 {
			return nil, fmt.Errorf("invalid sector %q", s.ID)
		}
		if err := knownAssets(s.Assets, assets); err != nil {
			return nil, fmt.Errorf("sector %s: %w", s.ID, err)
		}
		out[s.ID] = s
	}
	return out, nil
}
