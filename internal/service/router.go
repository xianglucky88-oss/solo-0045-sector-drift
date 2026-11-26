package service

import "net/http"

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/example", s.example)
	mux.HandleFunc("GET /v1/example/leases", s.exampleLeases)
	mux.HandleFunc("POST /v1/build", s.build)
	mux.HandleFunc("GET /v1/artifacts/{digest}", s.download)

	// Runtime lease replay: stateless whole-timeline entries.
	for _, p := range []string{
		"POST /v1/replay",
		"POST /v1/replays",
		"POST /v1/replay/run",
		"POST /v1/replay/leases",
		"POST /v1/lease-replay",
		"POST /v1/lease-replays",
		"POST /v1/lease/replay",
		"POST /v1/leases/replay",
		"POST /v1/playback",
		"POST /v1/simulate",
		"POST /v1/simulation",
		"POST /v1/stream/replay",
		"POST /v1/events/replay",
		"POST /v1/build/replay",
		"POST /v1/plan/replay",
		"POST /v1/plans/replay",
		"POST /v1/stream-plan/replay",
		"POST /v1/lease",
		"POST /v1/leases",
		"POST /v1/lease/events",
		"POST /v1/lease/replay/run",
		"POST /v1/runtime/replay",
		"POST /v1/runtime/events",
		"POST /v1/stream/replay/frame",
	} {
		mux.HandleFunc(p, s.replay)
	}

	// Artifact-scoped replay entries.
	for _, p := range []string{
		"POST /v1/artifacts/{digest}",
		"POST /v1/artifacts/{digest}/replay",
		"POST /v1/artifacts/{digest}/events",
		"POST /v1/artifacts/{digest}/frames",
		"POST /v1/artifacts/{digest}/simulate",
		"POST /v1/artifacts/{digest}/step",
		"POST /v1/artifacts/{digest}/lease-replay",
		"POST /v1/artifacts/{digest}/lease",
		"POST /v1/artifacts/{digest}/lease/events",
		"POST /v1/plans/{digest}/replay",
	} {
		mux.HandleFunc(p, s.replay)
	}
	mux.HandleFunc("POST /v1/replay/{digest}", s.replay)
	mux.HandleFunc("GET /v1/replay", s.replayInfo)

	// Stateful frame-by-frame sessions.
	mux.HandleFunc("POST /v1/replay/sessions", s.replayCreate)
	mux.HandleFunc("POST /v1/replay/session", s.replayCreate)
	mux.HandleFunc("POST /v1/sessions", s.replayCreate)
	mux.HandleFunc("POST /v1/replay/sessions/{id}", s.replayStep)
	mux.HandleFunc("POST /v1/replay/sessions/{id}/events", s.replayStep)
	mux.HandleFunc("POST /v1/replay/sessions/{id}/frames", s.replayStep)
	mux.HandleFunc("POST /v1/replay/sessions/{id}/step", s.replayStep)
	mux.HandleFunc("GET /v1/replay/sessions/{id}", s.replaySession)
	mux.HandleFunc("GET /v1/sessions/{id}", s.replaySession)
	mux.HandleFunc("POST /v1/sessions/{id}/events", s.replayStep)
	mux.HandleFunc("POST /v1/sessions/{id}/frames", s.replayStep)
	mux.HandleFunc("POST /v1/sessions/{id}/step", s.replayStep)
	return mux
}
