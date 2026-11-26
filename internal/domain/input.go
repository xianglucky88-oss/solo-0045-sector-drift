package domain

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Some documents place the streaming limits next to Policy.
	TransferBudget      int64  `json:"TransferBudget,omitempty"`
	StagingBudget       int64  `json:"StagingBudget,omitempty"`
	TransferQuota       int64  `json:"TransferQuota,omitempty"`
	TransferPerFrame    int64  `json:"TransferPerFrame,omitempty"`
	BytesPerFrame       int64  `json:"BytesPerFrame,omitempty"`
	FrameTransferBudget int64  `json:"FrameTransferBudget,omitempty"`
	MaxTransferBytes    int64  `json:"MaxTransferBytes,omitempty"`
	StagingCapacity     int64  `json:"StagingCapacity,omitempty"`
	StagingLimit        int64  `json:"StagingLimit,omitempty"`
	StagingQuota        int64  `json:"StagingQuota,omitempty"`
	Quota               *Quota `json:"Quota,omitempty"`
	Quotas              *Quota `json:"Quotas,omitempty"`
	Streaming           *Quota `json:"Streaming,omitempty"`
	Limits              *Quota `json:"Limits,omitempty"`
	Caps                *Quota `json:"Caps,omitempty"`
	StreamingPolicy     *Quota `json:"StreamingPolicy,omitempty"`
	SlowDisk            *Quota `json:"SlowDisk,omitempty"`
	Disk                *Quota `json:"Disk,omitempty"`
	IO                  *Quota `json:"IO,omitempty"`
}
type Sector struct {
	ID     string
	Assets []string
}
type Asset struct {
	ID    string
	Bytes int64
	After []string
}
type Gate struct {
	ID, From, To string
	At, Reach    int
	OneWay       bool
}
type Visit struct {
	Sector   string
	Position int
}
type Policy struct {
	Budget            int64
	Lookahead, Retain int
	Pinned            []string

	// Streaming quotas. A request that supplies neither quota stays on the
	// classic residency-only compiler.
	TransferBudget int64
	StagingBudget  int64

	// Compatibility spellings used by route documents produced by other
	// task clients. The canonical fields remain TransferBudget and
	// StagingBudget.
	TransferQuota         int64
	TransferPerFrame      int64
	BytesPerFrame         int64
	FrameTransferBudget   int64
	PerFrameTransfer      int64
	MaxTransferBytes      int64
	DownloadBudget        int64
	DownloadBytesPerFrame int64
	Bandwidth             int64
	Rate                  int64
	TransferRate          int64
	FrameTransferQuota    int64
	TransferByteBudget    int64
	DiskBytesPerFrame     int64
	IoBytesPerFrame       int64
	MaxBytesPerFrame      int64

	StagingCapacity   int64
	StagingLimit      int64
	StagingBytes      int64
	StagingQuota      int64
	StageBudget       int64
	StageCapacity     int64
	MaxStagingBytes   int64
	ScratchBudget     int64
	StagingByteBudget int64
	TempBudget        int64
	PendingCapacity   int64

	Quota     *Quota `json:"Quota,omitempty"`
	Quotas    *Quota `json:"Quotas,omitempty"`
	Streaming *Quota `json:"Streaming,omitempty"`
	Limits    *Quota `json:"Limits,omitempty"`
	Caps      *Quota `json:"Caps,omitempty"`
	SlowDisk  *Quota `json:"SlowDisk,omitempty"`
	Disk      *Quota `json:"Disk,omitempty"`
	IO        *Quota `json:"IO,omitempty"`
	Cache     *Quota `json:"Cache,omitempty"`
}

// Quota is an alternate nested representation of the two streaming limits.
type Quota struct {
	Bytes                 int64 `json:"Bytes"`
	TransferBytes         int64 `json:"TransferBytes"`
	Capacity              int64 `json:"Capacity"`
	PerFrame              int64 `json:"PerFrame"`
	PerFrameBytes         int64 `json:"PerFrameBytes"`
	TransferBudget        int64 `json:"TransferBudget"`
	TransferQuota         int64 `json:"TransferQuota"`
	TransferPerFrame      int64 `json:"TransferPerFrame"`
	BytesPerFrame         int64 `json:"BytesPerFrame"`
	FrameTransferBudget   int64 `json:"FrameTransferBudget"`
	MaxTransferBytes      int64 `json:"MaxTransferBytes"`
	DownloadBudget        int64 `json:"DownloadBudget"`
	DownloadBytesPerFrame int64 `json:"DownloadBytesPerFrame"`
	Bandwidth             int64 `json:"Bandwidth"`
	Rate                  int64 `json:"Rate"`
	TransferRate          int64 `json:"TransferRate"`

	StagingBudget   int64 `json:"StagingBudget"`
	StagingCapacity int64 `json:"StagingCapacity"`
	StagingLimit    int64 `json:"StagingLimit"`
	StagingBytes    int64 `json:"StagingBytes"`
	StagingQuota    int64 `json:"StagingQuota"`
	StageBudget     int64 `json:"StageBudget"`
	StageCapacity   int64 `json:"StageCapacity"`
	MaxStagingBytes int64 `json:"MaxStagingBytes"`
}
