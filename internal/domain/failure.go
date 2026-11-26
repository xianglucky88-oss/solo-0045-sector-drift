package domain

import "fmt"

// HandoverFailure is the diagnostic for a route that cannot be made
// handover-ready on every frame. Prefix is the number of initial frames that
// are executable; FailedFrame is the first frame that cannot hand over.
type HandoverFailure struct {
	Prefix            int      `json:"Prefix"`
	LongestPrefix     int      `json:"LongestPrefix"`
	CompletedFrames   int      `json:"CompletedFrames"`
	ThroughFrame      int      `json:"ThroughFrame"`
	NextFrame         int      `json:"NextFrame"`
	FailedFrame       int      `json:"FailedFrame"`
	BlockedFrame      int      `json:"BlockedFrame"`
	Frame             int      `json:"Frame"`
	At                int      `json:"At"`
	PrefixFrames      int      `json:"PrefixFrames"`
	Missing           []string `json:"Missing"`
	MissingAssets     []string `json:"MissingAssets"`
	MissingResources  []string `json:"MissingResources"`
	Unmet             []string `json:"Unmet"`
	Needed            []string `json:"Needed"`
	MissingAtFrameEnd []string `json:"MissingAtFrameEnd"`
}

// HandoverError carries the structured prefix diagnostic through the service
// boundary while still behaving as an ordinary domain rejection.
type HandoverError struct {
	Report         HandoverFailure
	Reason         string
	PrefixSchedule []Frame
	AttemptedFrame *Frame
}

func (e *HandoverError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("frame %d handover failed: %s", e.Report.FailedFrame, e.Reason)
	}
	return fmt.Sprintf("frame %d handover failed; %d required asset(s) missing", e.Report.FailedFrame, len(e.Report.Missing))
}

func newHandoverFailure(frame, total int, missing []string, reason string) *HandoverError {
	if missing == nil {
		missing = []string{}
	}
	f := HandoverFailure{
		Prefix:            frame,
		LongestPrefix:     frame,
		CompletedFrames:   frame,
		ThroughFrame:      frame - 1,
		NextFrame:         frame,
		FailedFrame:       frame,
		BlockedFrame:      frame,
		Frame:             frame,
		At:                frame,
		PrefixFrames:      frame,
		Missing:           missing,
		MissingAssets:     append([]string(nil), missing...),
		MissingResources:  append([]string(nil), missing...),
		Unmet:             append([]string(nil), missing...),
		Needed:            append([]string(nil), missing...),
		MissingAtFrameEnd: append([]string(nil), missing...),
	}
	_ = total
	return &HandoverError{Report: f, Reason: reason}
}
