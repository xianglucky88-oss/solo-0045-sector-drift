package domain

type Usage struct {
	PeakBytes             int64
	LoadedBytes           int64
	EvictedBytes          int64
	Deferred              int
	PeakStagingBytes      int64 `json:"PeakStagingBytes,omitempty"`
	PeakStaging           int64 `json:"PeakStaging,omitempty"`
	StagingPeakBytes      int64 `json:"StagingPeakBytes,omitempty"`
	StagingPeak           int64 `json:"StagingPeak,omitempty"`
	PeakStagedBytes       int64 `json:"PeakStagedBytes,omitempty"`
	TotalTransferBytes    int64 `json:"TotalTransferBytes,omitempty"`
	TotalTransferred      int64 `json:"TotalTransferred,omitempty"`
	TransferredBytes      int64 `json:"TransferredBytes,omitempty"`
	TransferBytes         int64 `json:"TransferBytes,omitempty"`
	DownloadedBytes       int64 `json:"DownloadedBytes,omitempty"`
	PeakTransferBytes     int64 `json:"PeakTransferBytes,omitempty"`
	PeakTransfer          int64 `json:"PeakTransfer,omitempty"`
	MaxFrameTransferBytes int64 `json:"MaxFrameTransferBytes,omitempty"`
}

func (u *Usage) record(f Frame, assets map[string]Asset) {
	if f.Bytes > u.PeakBytes {
		u.PeakBytes = f.Bytes
	}
	for _, op := range f.Operations {
		switch op.Action {
		case "load", "activate":
			u.LoadedBytes += assets[op.Asset].Bytes
		case "evict":
			u.EvictedBytes += assets[op.Asset].Bytes
		case "transfer", "download":
			n := op.Bytes
			if n == 0 {
				n = assets[op.Asset].Bytes
			}
			u.TotalTransferBytes += n
		}
	}
	for _, c := range f.Decisions {
		if !c.Admitted {
			u.Deferred++
		}
	}
}

func (u *Usage) recordStream(f Frame) {
	if f.StagingBytes > u.PeakStagingBytes {
		u.PeakStagingBytes = f.StagingBytes
	}
	u.PeakStaging = u.PeakStagingBytes
	if f.StagingBytes > u.StagingPeakBytes {
		u.StagingPeakBytes = f.StagingBytes
	}
	u.StagingPeak = u.StagingPeakBytes
	if f.StagingBytes > u.PeakStagedBytes {
		u.PeakStagedBytes = f.StagingBytes
	}
	if f.TransferBytes > u.PeakTransferBytes {
		u.PeakTransferBytes = f.TransferBytes
	}
	u.PeakTransfer = u.PeakTransferBytes
	if f.TransferBytes > u.MaxFrameTransferBytes {
		u.MaxFrameTransferBytes = f.TransferBytes
	}
	u.TotalTransferred = u.TotalTransferBytes
	u.TransferredBytes = u.TotalTransferBytes
	u.TransferBytes = u.TotalTransferBytes
	u.DownloadedBytes = u.TotalTransferBytes
}
