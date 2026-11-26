package service

import (
	"encoding/json"
	"net/http"

	"example.local/sector-drift/internal/domain"
)

// exampleLeases returns the synthetic demo augmented with a cutscene lease
// that keeps the bank actor/skeleton referenced after crossing into span.
func (s *Service) exampleLeases(w http.ResponseWriter, _ *http.Request) {
	var req domain.Request
	if err := json.Unmarshal(demo, &req); err != nil {
		problem(w, 500, "bad embedded example")
		return
	}
	req.Leases = domain.LeaseListForExample()
	writeJSON(w, 200, req)
}
