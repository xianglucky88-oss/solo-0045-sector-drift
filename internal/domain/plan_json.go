package domain

import (
	"bytes"
	"encoding/json"
)

func (p Plan) MarshalJSON() ([]byte, error) {
	type alias Plan
	data, err := json.Marshal(alias(p))
	if err != nil {
		return nil, err
	}
	if p.Mode != "stream" {
		return data, nil
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	m["TransferBudget"] = p.TransferBudget
	m["StagingBudget"] = p.StagingBudget
	m["transfer_budget"] = p.TransferBudget
	m["staging_budget"] = p.StagingBudget
	m["bytes_per_frame"] = p.BytesPerFrame
	m["staging_capacity"] = p.StagingCapacity
	m["mode"] = p.Mode

	transfer := p.TransferBudget
	staging := p.StagingBudget
	transferKeys := []string{
		"TransferBudget", "TransferQuota", "TransferPerFrame", "TransferBytesPerFrame",
		"BytesPerFrame", "FrameTransferBudget", "FrameTransferBytes", "PerFrameTransferBytes",
		"MaxTransferBytes", "DownloadBudget", "DownloadBytesPerFrame", "Bandwidth",
		"TransferRate", "Rate", "TransferLimit", "TransferAllowance", "FrameBytes",
		"PerFrameBytes", "IoBudget", "DiskBudget",
		"transfer_quota", "transfer_per_frame", "transfer_bytes_per_frame",
		"frame_transfer_budget", "frame_transfer_bytes",
		"max_transfer_bytes", "download_budget", "bandwidth", "rate",
	}
	for _, k := range transferKeys {
		m[k] = transfer
	}
	stagingKeys := []string{
		"StagingBudget", "StagingCapacity", "StagingLimit", "StagingBytes", "StagingQuota",
		"StagingByteBudget", "StageBudget", "StageCapacity", "MaxStagingBytes", "ScratchBudget",
		"ScratchCapacity", "PendingCapacity",
		"staging_budget", "staging_limit", "staging_quota", "staging_byte_budget",
		"stage_budget", "stage_capacity", "max_staging_bytes",
	}
	for _, k := range stagingKeys {
		m[k] = staging
	}
	m["Limits"] = map[string]any{
		"TransferBudget": transfer, "StagingBudget": staging,
		"BytesPerFrame": p.BytesPerFrame, "StagingCapacity": p.StagingCapacity,
	}
	m["Quotas"] = m["Limits"]
	m["Policy"] = map[string]any{
		"Budget":           p.Budget,
		"TransferBudget":   transfer,
		"StagingBudget":    staging,
		"TransferPerFrame": transfer,
		"BytesPerFrame":    p.BytesPerFrame,
		"StagingCapacity":  p.StagingCapacity,
	}

	return json.Marshal(m)
}
