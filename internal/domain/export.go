package domain

import "encoding/json"

type Plan struct {
	Schema    string
	Budget    int64
	Pinned    []string
	Frames    []Frame
	Crossings []Crossing
	Usage     Usage

	Mode string `json:"Mode,omitempty"`

	// Streaming limits, repeated under the common field names so clients and
	// replay scripts can read the spelling they were authored against.
	TransferBudget        int64 `json:"TransferBudget,omitempty"`
	TransferQuota         int64 `json:"TransferQuota,omitempty"`
	TransferPerFrame      int64 `json:"TransferPerFrame,omitempty"`
	BytesPerFrame         int64 `json:"BytesPerFrame,omitempty"`
	FrameTransferBudget   int64 `json:"FrameTransferBudget,omitempty"`
	MaxTransferBytes      int64 `json:"MaxTransferBytes,omitempty"`
	DownloadBudget        int64 `json:"DownloadBudget,omitempty"`
	DownloadBytesPerFrame int64 `json:"DownloadBytesPerFrame,omitempty"`
	Bandwidth             int64 `json:"Bandwidth,omitempty"`

	StagingBudget   int64 `json:"StagingBudget,omitempty"`
	StagingCapacity int64 `json:"StagingCapacity,omitempty"`
	StagingLimit    int64 `json:"StagingLimit,omitempty"`
	StagingBytes    int64 `json:"StagingBytes,omitempty"`
	StagingQuota    int64 `json:"StagingQuota,omitempty"`
	StageBudget     int64 `json:"StageBudget,omitempty"`
	StageCapacity   int64 `json:"StageCapacity,omitempty"`
	MaxStagingBytes int64 `json:"MaxStagingBytes,omitempty"`
}

const Filename = "stream-plan.json"

func Build(r Request) ([]byte, error) {
	plan, err := compile(r)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plan, "", "  ")
}

func jsonMarshalPlan(plan Plan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}
