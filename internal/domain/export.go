package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}
