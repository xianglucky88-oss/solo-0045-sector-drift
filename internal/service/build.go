package service

import (
	"encoding/json"
	"net/http"

	"example.local/sector-drift/internal/domain"
)

type Receipt struct {
	Digest   string `json:"digest"`
	Bytes    int    `json:"bytes"`
	Artifact string `json:"artifact"`
}

func (s *Service) build(w http.ResponseWriter, r *http.Request) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		problem(w, 503, "compiler busy")
		return
	}
	input, status, err := decode(w, r)
	if err != nil {
		problem(w, status, err.Error())
		return
	}
	bytes, err := compile(input)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	if r.Context().Err() != nil {
		return
	}
	digest, created, err := s.vault.Put(bytes)
	if err != nil {
		problem(w, 500, "cannot save artifact")
		return
	}
	location := "/v1/artifacts/" + digest
	w.Header().Set("Location", location)
	status = 200
	if created {
		status = 201
	}
	receipt := Receipt{digest, len(bytes), location}
	var plan domain.Plan
	_ = json.Unmarshal(bytes, &plan)
	if plan.Replay != nil {
		writeJSON(w, status, buildResponse{
			Receipt: receipt,
			Replay:  plan.Replay,
			Report:  plan.Replay,
			Result:  plan.Replay,
		})
		return
	}
	writeJSON(w, status, receipt)
}

type buildResponse struct {
	Receipt
	Replay *domain.ReplayReport `json:"Replay,omitempty"`
	Report *domain.ReplayReport `json:"Report,omitempty"`
	Result *domain.ReplayReport `json:"Result,omitempty"`
}
