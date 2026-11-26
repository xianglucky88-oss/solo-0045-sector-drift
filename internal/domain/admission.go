package domain

import "fmt"

type Decision struct {
	Candidate Candidate
	Admitted  bool
	Increment int64

	// Camera attribution (dual-camera plans only). Camera/Shot name the
	// camera this decision row belongs to; Cameras/Shots list every camera
	// that owns the same candidate sector.
	Camera   string   `json:"Camera,omitempty"`
	Shot     string   `json:"Shot,omitempty"`
	CameraID string   `json:"CameraID,omitempty"`
	ShotID   string   `json:"ShotID,omitempty"`
	Owner    string   `json:"Owner,omitempty"`
	Cameras  []string `json:"Cameras,omitempty"`
	Shots    []string `json:"Shots,omitempty"`
}

func admit(active, pinned Set, choices []Candidate, groups map[string]Set, assets map[string]Asset, budget int64) (Set, []Decision, error) {
	resident := make(Set)
	resident.merge(active)
	resident.merge(pinned)
	used := byteCost(resident, assets)
	if used > budget {
		return nil, nil, fmt.Errorf("mandatory residency %d exceeds %d bytes", used, budget)
	}
	decisions := make([]Decision, 0, len(choices))
	for _, c := range choices {
		cost := incremental(groups[c.Sector], resident, assets)
		allowed := cost <= budget-used
		if allowed {
			resident.merge(groups[c.Sector])
			used += cost
		}
		decisions = append(decisions, Decision{Candidate: c, Admitted: allowed, Increment: cost})
	}
	return resident, decisions, nil
}
