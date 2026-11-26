package service

import (
	"bytes"
	"errors"
	"example.local/sector-drift/internal/domain"
	"example.local/sector-drift/internal/vault"
	"io/fs"
	"net/http"
	"time"
)

func (s *Service) download(w http.ResponseWriter, r *http.Request) {
	digest := r.PathValue("digest")
	data, err := s.vault.Get(digest)
	switch {
	case errors.Is(err, vault.ErrDigest), errors.Is(err, fs.ErrNotExist):
		problem(w, 404, "artifact not found")
		return
	case errors.Is(err, vault.ErrIntegrity):
		problem(w, 409, "artifact integrity mismatch")
		return
	case err != nil:
		problem(w, 500, "cannot read artifact")
		return
	}

	// A drill selector on the artifact URL replays a fault against this plan.
	if r.Method == http.MethodGet && queryFrame(r) != nil {
		frame := *queryFrame(r)
		asset := queryAsset(r)
		if asset == "" {
			problem(w, http.StatusUnprocessableEntity, "drill query requires asset/member")
			return
		}
		report, derr := domain.DrillFromArtifact(data, frame, asset)
		if derr != nil {
			domainProblem(w, derr)
			return
		}
		out, _ := domain.MarshalDrill(report)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+domain.Filename+`"`)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+digest+`"`)
	http.ServeContent(w, r, domain.Filename, time.Time{}, bytes.NewReader(data))
}
