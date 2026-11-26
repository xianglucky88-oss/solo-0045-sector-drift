package service

import (
	"encoding/json"
	"errors"
	"example.local/sector-drift/internal/domain"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func problem(w http.ResponseWriter, status int, text string) {
	writeJSON(w, status, map[string]string{"error": text})
}

// domainProblem preserves the {"error": ...} contract while adding structured
// blocker/cycle fields when the domain error carries them.
func domainProblem(w http.ResponseWriter, err error) {
	body := map[string]any{"error": err.Error()}
	var block *domain.BlockError
	if errors.As(err, &block) {
		if block.Group != "" {
			body["Group"] = block.Group
			body["GroupID"] = block.Group
			body["Blocked"] = block.Group
			body["BlockedGroup"] = block.Group
			body["BlockedGroupID"] = block.Group
			body["BlockingGroup"] = block.Group
			body["Blocker"] = block.Group
		}
		if len(block.Groups) > 0 {
			body["Groups"] = block.Groups
			body["Blockers"] = block.Groups
			body["BlockingGroups"] = block.Groups
		}
		if block.Needed != 0 || block.Budget != 0 {
			body["Needed"] = block.Needed
			body["Budget"] = block.Budget
		}
	}
	var cycle *domain.CycleError
	if errors.As(err, &cycle) {
		body["CycleError"] = true
		if len(cycle.Groups) > 0 {
			body["CycleGroups"] = cycle.Groups
		}
		if len(cycle.Cycle) > 0 {
			body["Cycle"] = cycle.Cycle
		}
	}
	writeJSON(w, http.StatusUnprocessableEntity, body)
}
