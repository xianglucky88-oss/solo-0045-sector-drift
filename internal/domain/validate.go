package domain

import (
	"fmt"
	"regexp"
)

var label = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func validate(r Request) error {
	if err := validateCommon(r); err != nil {
		return err
	}
	if len(r.Route) < 1 || len(r.Route) > 2048 {
		return fmt.Errorf("invalid route count")
	}
	return nil
}

func validateCommon(r Request) error {
	if len(r.Sectors) < 1 || len(r.Sectors) > 128 || len(r.Assets) < 1 || len(r.Assets) > 512 || len(r.Gates) > 512 {
		return fmt.Errorf("invalid sector, asset or gate count")
	}
	if r.Policy.Budget < 1 || r.Policy.Budget > 1<<40 || r.Policy.Lookahead < 0 || r.Policy.Lookahead > 10000 || r.Policy.Retain < 0 || r.Policy.Retain > 64 {
		return fmt.Errorf("invalid streaming policy")
	}
	return nil
}

func validateSearch(s Search) error {
	if len(s.Signposts) < 1 || len(s.Signposts) > 2048 {
		return fmt.Errorf("invalid signpost count")
	}
	if len(s.Moves) > 65536 {
		return fmt.Errorf("invalid move count")
	}
	if s.MaxMoves < 0 || s.MaxMoves > 100000 {
		return fmt.Errorf("invalid move limit")
	}
	seen := make(Set)
	for _, sp := range s.Signposts {
		if sp.ID == "" {
			return fmt.Errorf("signpost is missing an id")
		}
		if seen[sp.ID] {
			return fmt.Errorf("duplicate signpost %q", sp.ID)
		}
		seen[sp.ID] = true
		if sp.Position < 0 || sp.Position > 10000 {
			return fmt.Errorf("signpost %q is outside the authored world", sp.ID)
		}
	}
	if !seen[s.Start] || !seen[s.End] {
		return fmt.Errorf("start or end signpost is not defined")
	}
	return nil
}
