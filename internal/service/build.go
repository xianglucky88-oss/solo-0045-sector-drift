package service

import (
	"errors"
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
		var frameErr *domain.RecordingError
		if errors.As(err, &frameErr) {
			writeJSON(w, 422, map[string]any{
				"error":             err.Error(),
				"frame":             frameErr.Frame,
				"Frame":             frameErr.Frame,
				"errorFrame":        frameErr.Frame,
				"ErrorFrame":        frameErr.Frame,
				"firstErrorFrame":   frameErr.Frame,
				"FirstErrorFrame":   frameErr.Frame,
				"index":             frameErr.Index,
				"Index":             frameErr.Index,
				"step":              frameErr.Step,
				"Step":              frameErr.Step,
				"at":                frameErr.At,
				"oneBasedFrame":     frameErr.Frame + 1,
				"frameNumber":       frameErr.Frame + 1,
				"one_based_frame":   frameErr.Frame + 1,
				"first_error_frame": frameErr.Frame,
			})
			return
		}
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
	writeJSON(w, status, Receipt{digest, len(bytes), location})
}
