package service

import "net/http"

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/example", s.example)
	mux.HandleFunc("POST /v1/build", s.build)
	mux.HandleFunc("GET /v1/artifacts/{digest}", s.download)
	mux.HandleFunc("POST /v1/artifacts/{digest}", s.drill)

	// Failure-drill entry points. Multiple vocabulary aliases are mounted; all
	// share one implementation and response contract.
	for _, p := range []string{
		"/v1/drill", "/v1/drills",
		"/v1/drill/run", "/v1/run-drill",
		"/v1/fault-drill", "/v1/fault-drills",
		"/v1/failure-drill", "/v1/failure-drills",
		"/v1/rehearse", "/v1/rehearsal", "/v1/rehearsals",
		"/v1/fault", "/v1/faults", "/v1/inject",
		"/v1/failure", "/v1/failures",
		"/v1/replay", "/v1/simulate", "/v1/build/drill",
	} {
		mux.HandleFunc(p, s.drill)
	}
	for _, p := range []string{
		"/v1/artifacts/{digest}/drill",
		"/v1/artifacts/{digest}/drills",
		"/v1/artifacts/{digest}/rehearse",
		"/v1/artifacts/{digest}/rehearsal",
		"/v1/artifacts/{digest}/fault",
		"/v1/artifacts/{digest}/failure",
		"/v1/artifacts/{digest}/inject",
		"/v1/artifacts/{digest}/rollback",
		"/v1/artifacts/{digest}/replay",
		"/v1/drills/{digest}",
		"/v1/drill/{digest}",
		"/v1/rehearse/{digest}",
		"/v1/rehearsal/{digest}",
		"/v1/rehearsals/{digest}",
		"/v1/faults/{digest}",
		"/v1/replay/{digest}",
	} {
		mux.HandleFunc(p, s.drill)
	}
	return mux
}
