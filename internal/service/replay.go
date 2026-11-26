package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"example.local/sector-drift/internal/domain"
	"example.local/sector-drift/internal/vault"
	"io/fs"
)

type sessionEntry struct {
	mu      sync.Mutex
	stepper *domain.Stepper
	created time.Time
}

type sessionHub struct {
	mu sync.Mutex
	m  map[string]*sessionEntry
}

func newSessionHub() *sessionHub { return &sessionHub{m: make(map[string]*sessionEntry)} }

func (h *sessionHub) put(id string, e *sessionEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.m) > 512 {
		var oldest string
		var oldestT time.Time
		for k, v := range h.m {
			if oldest == "" || v.created.Before(oldestT) {
				oldest, oldestT = k, v.created
			}
		}
		delete(h.m, oldest)
	}
	h.m[id] = e
}

func (h *sessionHub) get(id string) (*sessionEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	e, ok := h.m[id]
	return e, ok
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// decodeReplayBody reads a lenient replay request. An empty body is valid.
func decodeReplayBody(r *http.Request) (domain.ReplayRequest, error) {
	var rr domain.ReplayRequest
	if r.Body == nil {
		return rr, nil
	}
	data, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 8<<20))
	if err != nil {
		return rr, err
	}
	if len(data) == 0 {
		return rr, nil
	}
	if err := json.Unmarshal(data, &rr); err != nil {
		return rr, err
	}
	return rr, nil
}

// resolvePlan finds the stream plan from an embedded request/plan or the vault.
func (s *Service) resolvePlan(w http.ResponseWriter, r *http.Request, rr *domain.ReplayRequest, pathDigest string) (*domain.Plan, bool) {
	if pathDigest != "" {
		rr.Digest = pathDigest
	}
	if rr.Request != nil {
		blob, err := domain.Build(*rr.Request)
		if err != nil {
			problem(w, 422, err.Error())
			return nil, false
		}
		var p domain.Plan
		if err := json.Unmarshal(blob, &p); err != nil {
			problem(w, 500, "cannot compile request")
			return nil, false
		}
		if rr.Digest == "" {
			rr.Digest = "inline-request"
		}
		return &p, true
	}
	if rr.Plan != nil {
		if rr.Digest == "" {
			rr.Digest = "inline-artifact"
		}
		return rr.Plan, true
	}
	if rr.Digest != "" {
		data, err := s.vault.Get(rr.Digest)
		switch {
		case errors.Is(err, vault.ErrDigest), errors.Is(err, fs.ErrNotExist):
			problem(w, 404, "artifact not found")
			return nil, false
		case errors.Is(err, vault.ErrIntegrity):
			problem(w, 409, "artifact integrity mismatch")
			return nil, false
		case err != nil:
			problem(w, 500, "cannot read artifact")
			return nil, false
		}
		var p domain.Plan
		if err := json.Unmarshal(data, &p); err != nil {
			problem(w, 422, "artifact is not a stream plan")
			return nil, false
		}
		return &p, true
	}
	problem(w, 400, "missing artifact digest, embedded Artifact or Request")
	return nil, false
}

// replay handles stateless whole-timeline replay.
func (s *Service) replay(w http.ResponseWriter, r *http.Request) {
	rr, err := decodeReplayBody(r)
	if err != nil {
		problem(w, 400, "invalid replay document")
		return
	}
	if d := r.PathValue("digest"); d != "" {
		rr.Digest = d
	}
	if rr.Digest == "" {
		for _, q := range []string{"digest", "id", "artifact", "ref"} {
			if v := r.URL.Query().Get(q); v != "" {
				rr.Digest = domain.DigestFromRef(v)
				break
			}
		}
	}
	plan, ok := s.resolvePlan(w, r, &rr, rr.Digest)
	if !ok {
		return
	}
	report, err := domain.RunReplay(plan, rr.Leases, rr)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	writeReplayEnvelope(w, report)
}

// writeReplayEnvelope keeps the report's fields at the top level while also
// exposing it under the wrapper names different clients expect.
func writeReplayEnvelope(w http.ResponseWriter, report *domain.ReplayReport) {
	var top map[string]any
	raw, err := json.Marshal(report)
	if err == nil {
		_ = json.Unmarshal(raw, &top)
	}
	if top == nil {
		top = map[string]any{}
	}
	for _, k := range []string{"Report", "Result", "Replay", "Data", "Response", "Outcome"} {
		top[k] = report
	}
	writeJSON(w, 200, top)
}

// replayInfo describes the replay entry point for human/tool discovery.
func (s *Service) replayInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"endpoint": "/v1/replay",
		"usage":    `POST {"Digest":"<artifact digest>","Events":[{"Frame":2,"Credential":"cue","Type":"acquire"}]}`,
		"artifactScoped": []string{
			"POST /v1/artifacts/{digest}/replay",
			"POST /v1/replay/sessions",
		},
	})
}

// replayCreate starts a stateful frame-by-frame session.
func (s *Service) replayCreate(w http.ResponseWriter, r *http.Request) {
	rr, err := decodeReplayBody(r)
	if err != nil {
		problem(w, 400, "invalid replay document")
		return
	}
	if d := r.PathValue("digest"); d != "" {
		rr.Digest = d
	}
	plan, ok := s.resolvePlan(w, r, &rr, rr.Digest)
	if !ok {
		return
	}
	st, err := domain.NewStepper(plan, rr.Leases, rr.Digest)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	id := newID()
	s.sessions.put(id, &sessionEntry{stepper: st, created: time.Now()})

	// Optionally process the first frame in the same call.
	var first *domain.ReplayFrame
	events := allStepEvents(rr)
	if rr.HasFrame || len(events) > 0 {
		f, err := st.AdvanceTo(rr.Frame, events)
		if err != nil {
			problem(w, 422, err.Error())
			return
		}
		first = f
	}
	resp := map[string]any{
		"SessionID": id, "ID": id, "Session": id, "ReplayID": id,
		"Digest": rr.Digest, "FrameCount": len(plan.Frames),
		"NextFrame": st.Next,
		"Report":    st.Report(),
	}
	if first != nil {
		resp["Frame"] = first.Index
		resp["Result"] = first
		resp["Step"] = first
		resp["Operations"] = first.Operations
		resp["Resident"] = first.Resident
		resp["Results"] = first.Results
	}
	writeJSON(w, 201, resp)
}

// replayStep advances one frame in an existing session.
func (s *Service) replayStep(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ent, ok := s.sessions.get(id)
	if !ok {
		problem(w, 404, "replay session not found")
		return
	}
	rr, err := decodeReplayBody(r)
	if err != nil {
		problem(w, 400, "invalid replay document")
		return
	}
	events := allStepEvents(rr)
	frame := -1
	if rr.HasFrame {
		frame = rr.Frame
	}
	ent.mu.Lock()
	defer ent.mu.Unlock()
	f, err := ent.stepper.AdvanceTo(frame, events)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	report := ent.stepper.Report()
	writeJSON(w, 200, stepResponse{
		ReplayFrame: f,
		SessionID:   id, ID: id,
		NextFrame: ent.stepper.Next,
		Report:    report,
		Frames:    report.Frames,
	})
}

type stepResponse struct {
	*domain.ReplayFrame
	SessionID string               `json:"SessionID"`
	ID        string               `json:"ID"`
	NextFrame int                  `json:"NextFrame"`
	Report    *domain.ReplayReport `json:"Report"`
	Frames    []domain.ReplayFrame `json:"Frames"`
}

// replaySession returns the accumulated report.
func (s *Service) replaySession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ent, ok := s.sessions.get(id)
	if !ok {
		problem(w, 404, "replay session not found")
		return
	}
	ent.mu.Lock()
	defer ent.mu.Unlock()
	rep := ent.stepper.Report()
	writeJSON(w, 200, map[string]any{
		"SessionID": id, "ID": id, "NextFrame": ent.stepper.Next,
		"Report": rep, "Frames": rep.Frames, "Results": rep.Results,
	})
}

func allStepEvents(rr domain.ReplayRequest) []domain.LeaseEvent {
	out := append([]domain.LeaseEvent(nil), rr.Events...)
	for _, b := range rr.Buckets {
		for _, ev := range b.Events {
			if ev.Frame == 0 && !ev.HasFrame {
				ev.Frame, ev.HasFrame = b.Frame, true
			}
			out = append(out, ev)
		}
	}
	return out
}
