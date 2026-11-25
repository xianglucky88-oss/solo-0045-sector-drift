package domain

import (
	"fmt"
	"sort"
)

func dependencyOrder(assets map[string]Asset) ([]string, error) {
	marks := make(map[string]uint8)
	roots := make(Set)
	for id := range assets {
		roots[id] = true
	}
	order := make([]string, 0, len(assets))
	var visit func(string) error
	visit = func(id string) error {
		switch marks[id] {
		case 1:
			return fmt.Errorf("dependency cycle at %s", id)
		case 2:
			return nil
		}
		marks[id] = 1
		children := append([]string(nil), assets[id].After...)
		sort.Strings(children)
		for _, d := range children {
			if err := visit(d); err != nil {
				return err
			}
		}
		marks[id] = 2
		order = append(order, id)
		return nil
	}
	for _, id := range roots.sorted() {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return order, nil
}
