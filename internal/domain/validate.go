package domain

import (
	"fmt"
	"regexp"
)

var label = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func validate(r Request) error {
	if len(r.Sectors) < 1 || len(r.Sectors) > 128 || len(r.Assets) < 1 || len(r.Assets) > 512 || len(r.Gates) > 512 || len(r.Route) < 1 || len(r.Route) > 2048 {
		return fmt.Errorf("invalid sector, asset, gate or route count")
	}
	if r.Policy.Budget < 1 || r.Policy.Budget > 1<<40 || r.Policy.Lookahead < 0 || r.Policy.Lookahead > 10000 || r.Policy.Retain < 0 || r.Policy.Retain > 64 {
		return fmt.Errorf("invalid streaming policy")
	}
	return nil
}
