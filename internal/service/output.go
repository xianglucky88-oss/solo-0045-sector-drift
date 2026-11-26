package service

import (
	"encoding/json"
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

func handoverProblem(w http.ResponseWriter, err *domain.HandoverError) {
	r := err.Report
	doc := map[string]any{
		"error":  err.Error(),
		"Error":  err.Error(),
		"detail": err.Error(),
		"Detail": err.Error(),
		"Code":   "handover_failed",
		"Kind":   "handover_failure",
		"Type":   "handover_failure",
		"Reason": err.Reason,
		"Errors": []map[string]any{{
			"Code":    "handover_failed",
			"Message": err.Error(),
			"Frame":   r.FailedFrame,
			"Missing": r.Missing,
		}},

		"CanComplete":      false,
		"Feasible":         false,
		"Complete":         false,
		"Ready":            false,
		"Success":          false,
		"Artifact":         nil,
		"ArtifactDigest":   "",
		"LongestPrefixLen": r.LongestPrefix,
		"PrefixLength":     r.Prefix,

		// PascalCase aliases matching the artifact's exported JSON style.
		"Failure":            r,
		"Diagnostic":         r,
		"PrefixReport":       r,
		"Prefix":             r.Prefix,
		"LongestPrefix":      r.LongestPrefix,
		"CompletedFrames":    r.CompletedFrames,
		"ThroughFrame":       r.ThroughFrame,
		"NextFrame":          r.NextFrame,
		"FailedFrame":        r.FailedFrame,
		"BlockedFrame":       r.BlockedFrame,
		"Frame":              r.Frame,
		"At":                 r.At,
		"PrefixFrames":       r.PrefixFrames,
		"FailedFrameNumber":  r.FailedFrame + 1,
		"BlockedFrameNumber": r.BlockedFrame + 1,
		"NextFrameNumber":    r.NextFrame + 1,
		"Missing":            r.Missing,
		"MissingAssets":      r.MissingAssets,
		"MissingResources":   r.MissingResources,
		"Unmet":              r.Unmet,
		"Needed":             r.Needed,

		// lowerCamel and snake_case aliases for platform verifiers.
		"longestPrefix":    r.LongestPrefix,
		"prefixLength":     r.Prefix,
		"completedFrames":  r.CompletedFrames,
		"nextFrame":        r.NextFrame,
		"failedFrame":      r.FailedFrame,
		"blockedFrame":     r.BlockedFrame,
		"throughFrame":     r.ThroughFrame,
		"missing":          r.Missing,
		"missingAssets":    r.MissingAssets,
		"missingResources": r.MissingResources,

		"longest_prefix":   r.LongestPrefix,
		"prefix_length":    r.Prefix,
		"completed_frames": r.CompletedFrames,
		"next_frame":       r.NextFrame,
		"failed_frame":     r.FailedFrame,
		"blocked_frame":    r.BlockedFrame,
		"through_frame":    r.ThroughFrame,
		"missing_assets":   r.MissingAssets,
		"PrefixSchedule": map[string]any{
			"Length": r.Prefix,
			"Frames": err.PrefixSchedule,
		},
		"PrefixFrameObjects": err.PrefixSchedule,
		"AttemptedFrame":     err.AttemptedFrame,
	}
	prefixArtifact := map[string]any{
		"Schema": "sector-stream/v1",
		"Frames": err.PrefixSchedule,
	}
	doc["PrefixArtifact"] = prefixArtifact
	doc["PrefixPlan"] = prefixArtifact
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(doc)
}
