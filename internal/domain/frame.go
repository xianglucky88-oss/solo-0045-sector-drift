package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// Streaming-mode accounting. These fields are omitted in classic plans so
	// the established artifact bytes remain unchanged.
	Ready              bool     `json:"Ready,omitempty"`
	Complete           bool     `json:"Complete,omitempty"`
	HandedOff          bool     `json:"HandedOff,omitempty"`
	TransferBytes      int64    `json:"TransferBytes,omitempty"`
	Transferred        int64    `json:"Transferred,omitempty"`
	BytesTransferred   int64    `json:"BytesTransferred,omitempty"`
	FrameTransferBytes int64    `json:"FrameTransferBytes,omitempty"`
	DownloadBytes      int64    `json:"DownloadBytes,omitempty"`
	Staging            []string `json:"Staging,omitempty"`
	Staged             []string `json:"Staged,omitempty"`
	StagingBytes       int64    `json:"StagingBytes,omitempty"`
	StagedBytes        int64    `json:"StagedBytes,omitempty"`
	StagingUsed        int64    `json:"StagingUsed,omitempty"`
	Missing            []string `json:"Missing,omitempty"`

	// Detailed transfer instructions are redundant with transfer entries in
	// Operations, but give replay tools a stable named collection.
	Transfers []Op `json:"Transfers,omitempty"`
	Downloads []Op `json:"Downloads,omitempty"`
}
