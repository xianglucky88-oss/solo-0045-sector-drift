package service

import (
	"net/http"
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
	// A digest reference plus a fault selector is a drill against an existing
	// artifact, even when posted to /v1/build.
	if _, _, requested, terr := input.DrillTarget(); terr == nil && requested {
		digest := firstDigest(input.Digest, input.Artifact, input.ArtifactID, input.Ref)
		if digest != "" && !looksLikeFullSpec(&input) {
			s.respondDrill(w, r, input, nil, nil)
			return
		}
	}
	bytes, report, err := compileEx(input)
	if err != nil {
		domainProblem(w, err)
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
	if report != nil {
		// Inline the drill facts next to the receipt while keeping the receipt
		// fields identical for generic clients.
		payload := drillPayload(report, digest, len(bytes), location)
		writeJSON(w, status, payload)
		return
	}
	writeJSON(w, status, Receipt{digest, len(bytes), location})
}
