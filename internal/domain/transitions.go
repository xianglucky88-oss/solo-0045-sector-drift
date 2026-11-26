package domain

type Crossing struct {
	Step int
	Gate string
	From string
	To   string

	// Camera attribution (dual-camera plans only; omitted in legacy plans).
	Camera   string `json:"Camera,omitempty"`
	Shot     string `json:"Shot,omitempty"`
	CameraID string `json:"CameraID,omitempty"`
	ShotID   string `json:"ShotID,omitempty"`
	Owner    string `json:"Owner,omitempty"`
	By       string `json:"By,omitempty"`
}

func crossings(route []Visit, gates map[string][]Gate) []Crossing {
	out := make([]Crossing, 0)
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		if a.Sector == b.Sector {
			continue
		}
		for _, g := range gates[a.Sector] {
			if g.To == b.Sector && abs(a.Position-g.At) <= g.Reach {
				out = append(out, Crossing{Step: i, Gate: g.ID, From: a.Sector, To: b.Sector})
				break
			}
		}
	}
	return out
}
