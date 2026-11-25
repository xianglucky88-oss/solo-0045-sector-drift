package domain

func byteCost(ids Set, assets map[string]Asset) int64 { return incremental(ids, nil, assets) }
func incremental(group, existing Set, assets map[string]Asset) int64 {
	var bytes int64
	for id := range group {
		if !existing[id] {
			bytes += assets[id].Bytes
		}
	}
	return bytes
}
