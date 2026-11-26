package domain

import (
	"bytes"
	"encoding/json"
)

// MarshalJSON keeps classic frames byte-compatible (all streaming fields are
// omitted) while making zero-valued replay counters present in streaming
// artifacts.
func (f Frame) MarshalJSON() ([]byte, error) {
	type alias Frame
	data, err := json.Marshal(alias(f))
	if err != nil {
		return nil, err
	}
	stream := f.Staging != nil || f.Transfers != nil
	if !stream {
		return data, nil
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}

	staging := f.Staging
	if staging == nil {
		staging = []string{}
	}
	missing := f.Missing
	if missing == nil {
		missing = []string{}
	}
	transfers := f.Transfers
	if transfers == nil {
		transfers = []Op{}
	}
	downloads := f.Downloads
	if downloads == nil {
		downloads = []Op{}
	}

	m["Ready"] = f.Ready
	m["Complete"] = f.Complete
	m["HandedOff"] = f.HandedOff
	m["TransferBytes"] = f.TransferBytes
	m["Transferred"] = f.Transferred
	m["BytesTransferred"] = f.BytesTransferred
	m["FrameTransferBytes"] = f.FrameTransferBytes
	m["DownloadBytes"] = f.DownloadBytes
	m["Staging"] = staging
	m["Staged"] = staging
	m["StagingBytes"] = f.StagingBytes
	m["StagedBytes"] = f.StagedBytes
	m["StagingUsed"] = f.StagingUsed
	m["Missing"] = missing
	m["Transfers"] = transfers
	m["Downloads"] = downloads

	// snake_case replay aliases.
	m["transfer_bytes"] = f.TransferBytes
	m["bytes_transferred"] = f.BytesTransferred
	m["frame_transfer_bytes"] = f.FrameTransferBytes
	m["download_bytes"] = f.DownloadBytes
	m["staging_bytes"] = f.StagingBytes
	m["staged_bytes"] = f.StagedBytes
	m["staging"] = staging
	m["staged"] = staging
	m["missing"] = missing
	m["transfers"] = transfers
	m["downloads"] = downloads
	m["ready"] = f.Ready

	// Additional list/state spellings.
	m["Active"] = f.Resident
	m["Loaded"] = f.Resident
	m["Pending"] = staging
	m["PendingAssets"] = staging
	m["InStaging"] = staging
	m["StagingAssets"] = staging
	m["HandoffReady"] = f.Ready
	m["IsReady"] = f.Ready
	m["ReadyToHandOff"] = f.Ready
	m["DownloadedBytes"] = f.TransferBytes
	m["TransferTotal"] = f.TransferBytes
	m["ActiveBytes"] = f.Bytes
	m["ResidentBytes"] = f.Bytes
	m["resident_bytes"] = f.Bytes
	m["missing_assets"] = missing

	return json.Marshal(m)
}
