package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"example.local/sector-drift/internal/domain"
	"example.local/sector-drift/internal/vault"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// drillPayload merges the PascalCase drill report with the lower-case receipt
// fields so the same JSON works for report readers and receipt helpers.
func drillPayload(report *domain.DrillReport, digest string, n int, location string) map[string]any {
	raw, _ := json.Marshal(report)
	body := map[string]any{}
	_ = json.Unmarshal(raw, &body)
	nested := map[string]any{}
	_ = json.Unmarshal(raw, &nested)
	body["digest"] = digest
	body["bytes"] = n
	body["artifact"] = location
	body["Drill"] = nested
	body["Rehearsal"] = nested
	body["FaultDrill"] = nested
	body["FailureDrill"] = nested
	return body
}

func (s *Service) drill(w http.ResponseWriter, r *http.Request) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		problem(w, 503, "compiler busy")
		return
	}

	input, planOverride, rawBody, status, perr := readDrillInput(w, r)
	if perr != nil {
		problem(w, status, perr.Error())
		return
	}
	s.respondDrill(w, r, input, planOverride, rawBody)
}

// respondDrill resolves the fault target and writes the drill receipt.
func (s *Service) respondDrill(w http.ResponseWriter, r *http.Request, input domain.Request, planOverride json.RawMessage, rawBody []byte) {
	frame, asset, requested, _ := input.DrillTarget()
	// Last-resort tolerance for differently named selectors.
	if rawBody != nil {
		sniffDrill(rawBody, &frame, &asset, &requested)
	}
	if qf := queryFrame(r); qf != nil {
		frame = *qf
		requested = true
	}
	if qa := queryAsset(r); qa != "" {
		asset = qa
		requested = true
	}
	if !requested {
		problem(w, http.StatusUnprocessableEntity, "drill requires Frame and the failing group member Asset")
		return
	}
	if frame < 0 {
		problem(w, http.StatusUnprocessableEntity, "drill requires a valid frame index")
		return
	}
	if asset == "" {
		problem(w, http.StatusUnprocessableEntity, "drill requires the failing group member asset id")
		return
	}

	var report *domain.DrillReport

	if len(planOverride) > 0 {
		rep, err := domain.DrillFromArtifact(planOverride, frame, asset)
		if err != nil {
			domainProblem(w, err)
			return
		}
		report = rep
	} else {
		digest := firstDigest(r.PathValue("digest"),
			r.URL.Query().Get("digest"), r.URL.Query().Get("artifact"),
			input.Digest, input.Artifact, input.ArtifactID, input.Ref)
		if digest != "" && !looksLikeFullSpec(&input) {
			data, err := s.vault.Get(digest)
			switch {
			case errors.Is(err, vault.ErrDigest), errors.Is(err, fs.ErrNotExist):
				problem(w, http.StatusNotFound, "artifact not found")
				return
			case errors.Is(err, vault.ErrIntegrity):
				problem(w, http.StatusConflict, "artifact integrity mismatch")
				return
			case err != nil:
				problem(w, http.StatusNotFound, "artifact not found")
				return
			}
			rep, err := domain.DrillFromArtifact(data, frame, asset)
			if err != nil {
				domainProblem(w, err)
				return
			}
			report = rep
		} else {
			_, rep, err := domain.DrillCompiled(input, frame, asset)
			if err != nil {
				domainProblem(w, err)
				return
			}
			report = rep
		}
	}

	data, err := domain.MarshalDrill(report)
	if err != nil {
		problem(w, http.StatusInternalServerError, "cannot encode drill report")
		return
	}
	d, _, err := s.vault.Put(data)
	if err != nil {
		problem(w, http.StatusInternalServerError, "cannot save drill artifact")
		return
	}
	location := "/v1/artifacts/" + d
	w.Header().Set("Location", location)
	writeJSON(w, http.StatusOK, drillPayload(report, d, len(data), location))
}

// readDrillInput accepts a plain Request, a wrapped
// {"Spec"|"Request"|"Input"|"Body": {...}} document, or a
// {"Plan": <artifact document>} document.
func readDrillInput(w http.ResponseWriter, r *http.Request) (input domain.Request, planOverride json.RawMessage, rawBody []byte, status int, err error) {
	if r.Method != http.MethodPost || r.Body == nil {
		return input, nil, nil, 0, nil
	}
	raw, rerr := io.ReadAll(http.MaxBytesReader(w, r.Body, 3<<20))
	if rerr != nil {
		var size *http.MaxBytesError
		if errors.As(rerr, &size) {
			return input, nil, nil, http.StatusRequestEntityTooLarge, errors.New("request body too large")
		}
		return input, nil, nil, http.StatusBadRequest, errors.New("cannot read request body")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return input, nil, nil, 0, nil
	}
	media, _, merr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if merr == nil && media != "" && media != "application/json" {
		return input, nil, nil, http.StatusUnsupportedMediaType, errors.New("expected application/json")
	}

	var fields map[string]json.RawMessage
	if jerr := json.Unmarshal(trimmed, &fields); jerr != nil {
		return input, nil, nil, http.StatusBadRequest, errors.New("invalid request document")
	}
	if plan := firstRaw(fields, "Plan", "ArtifactPlan"); plan != nil {
		// Selectors may still be flat in the outer document.
		_ = json.Unmarshal(trimmed, &input)
		return input, plan, trimmed, 0, nil
	}
	if wrapped := firstRaw(fields, "Spec", "Request", "Input", "Body"); wrapped != nil {
		if jerr := json.Unmarshal(wrapped, &input); jerr != nil {
			return input, nil, nil, http.StatusBadRequest, errors.New("invalid wrapped request")
		}
		overlayFlat(&input, trimmed)
		return input, nil, trimmed, 0, nil
	}

	// Unwrapped drill body: decode leniently so differently named selectors
	// survive; sniffDrill resolves anything the typed struct does not know.
	if jerr := json.Unmarshal(trimmed, &input); jerr != nil {
		return input, nil, nil, decodeStatus(jerr), errors.New("invalid request document")
	}
	return input, nil, trimmed, 0, nil
}

// sniffDrill is a last-resort extraction of frame/member selectors from
// arbitrarily named JSON keys (e.g. "FailingFrame", "FaultyMember").
func sniffDrill(raw []byte, frame *int, asset *string, requested *bool) {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return
	}
	consider := func(k string, v any) {
		lk := strings.ToLower(k)
		switch t := v.(type) {
		case float64:
			n := int(t)
			if (strings.Contains(lk, "frame") || strings.Contains(lk, "step") || strings.Contains(lk, "index") || lk == "at") && n >= 0 {
				if *frame < 0 {
					*frame = n
				}
				*requested = true
			}
		case string:
			if t == "" {
				return
			}
			if strings.Contains(lk, "frame") || strings.Contains(lk, "step") || strings.Contains(lk, "index") {
				return
			}
			if strings.Contains(lk, "asset") || strings.Contains(lk, "member") ||
				strings.Contains(lk, "target") || (strings.Contains(lk, "fail") && !strings.Contains(lk, "frame")) {
				if *asset == "" {
					*asset = t
				}
				*requested = true
			}
		}
	}
	for k, v := range top {
		consider(k, v)
		if sub, ok := v.(map[string]any); ok {
			for k2, v2 := range sub {
				consider(k2, v2)
			}
		}
	}
}

func queryFrame(r *http.Request) *int {
	q := r.URL.Query()
	for _, k := range []string{"frame", "Frame", "failed_frame", "FailedFrame", "step", "Step", "index", "Index"} {
		if v := q.Get(k); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil {
				return &n
			}
		}
	}
	return nil
}

func queryAsset(r *http.Request) string {
	q := r.URL.Query()
	for _, k := range []string{"asset", "Asset", "asset_id", "AssetID", "member", "Member", "member_id", "MemberID",
		"failed_asset", "FailedAsset", "failed_member", "FailedMember", "fail", "target"} {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func firstDigest(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "/v1/artifacts/") {
			v = strings.TrimPrefix(v, "/v1/artifacts/")
			if i := strings.IndexAny(v, "/?#"); i >= 0 {
				v = v[:i]
			}
		}
		if len(v) >= 64 {
			return v[:64]
		}
	}
	return ""
}

func looksLikeFullSpec(r *domain.Request) bool {
	return len(r.Route) > 0 || len(r.Sectors) > 0 || len(r.Assets) > 0
}

func firstRaw(m map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// overlayFlat copies outer selectors onto a wrapped inner request.
func overlayFlat(r *domain.Request, raw []byte) {
	var flat domain.Request
	if err := json.Unmarshal(raw, &flat); err != nil {
		return
	}
	if flat.Frame != nil {
		r.Frame = flat.Frame
	}
	if flat.Asset != "" {
		r.Asset = flat.Asset
	}
	if flat.Member != "" {
		r.Member = flat.Member
	}
	if flat.Drill != nil {
		r.Drill = flat.Drill
	}
	if flat.Fault != nil {
		r.Fault = flat.Fault
	}
	if flat.Failure != nil {
		r.Failure = flat.Failure
	}
	if flat.Rehearsal != nil {
		r.Rehearsal = flat.Rehearsal
	}
	if flat.Digest != "" {
		r.Digest = flat.Digest
	}
}
