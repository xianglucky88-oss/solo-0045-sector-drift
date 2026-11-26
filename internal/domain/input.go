package domain

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Recording is the preferred continuation input. The remaining fields are
	// compatibility aliases for callers that name the captured timeline differently.
	Recording *Recording `json:"Recording,omitempty"`
	Replay    *Recording `json:"Replay,omitempty"`
	Capture   *Recording `json:"Capture,omitempty"`

	Record            *Recording `json:"Record,omitempty"`
	Live              *Recording `json:"Live,omitempty"`
	LiveRecord        *Recording `json:"LiveRecord,omitempty"`
	LiveRecording     *Recording `json:"LiveRecording,omitempty"`
	Actual            *Recording `json:"Actual,omitempty"`
	ActualRecord      *Recording `json:"ActualRecord,omitempty"`
	ActualRecording   *Recording `json:"ActualRecording,omitempty"`
	Trace             *Recording `json:"Trace,omitempty"`
	LiveTrace         *Recording `json:"LiveTrace,omitempty"`
	EngineTrace       *Recording `json:"EngineTrace,omitempty"`
	RecordLog         *Recording `json:"RecordLog,omitempty"`
	Timeline          *Recording `json:"Timeline,omitempty"`
	History           *Recording `json:"History,omitempty"`
	Checkpoint        *Recording `json:"Checkpoint,omitempty"`
	Resume            *Recording `json:"Resume,omitempty"`
	Continuation      *Recording `json:"Continuation,omitempty"`
	Session           *Recording `json:"Session,omitempty"`
	ReplayRecord      *Recording `json:"ReplayRecord,omitempty"`
	RecordedTimeline  *Recording `json:"RecordedTimeline,omitempty"`
	ExecutionRecord   *Recording `json:"ExecutionRecord,omitempty"`
	EngineRecord      *Recording `json:"EngineRecord,omitempty"`
	RuntimeRecord     *Recording `json:"RuntimeRecord,omitempty"`
	PlaybackRecording *Recording `json:"PlaybackRecording,omitempty"`

	// Equivalent flat request representations.
	RecordedFrames []RecordedFrame `json:"RecordedFrames,omitempty"`
	LiveFrames     []RecordedFrame `json:"LiveFrames,omitempty"`
	ActualFrames   []RecordedFrame `json:"ActualFrames,omitempty"`
	TraceFrames    []RecordedFrame `json:"TraceFrames,omitempty"`
	FrameLog       []RecordedFrame `json:"FrameLog,omitempty"`

	Resident          []string `json:"Resident,omitempty"`
	Residency         []string `json:"Residency,omitempty"`
	ResidentSet       []string `json:"ResidentSet,omitempty"`
	PauseSet          []string `json:"PauseSet,omitempty"`
	PausedSet         []string `json:"PausedSet,omitempty"`
	PausedResident    []string `json:"PausedResident,omitempty"`
	PauseResident     []string `json:"PauseResident,omitempty"`
	PauseResidency    []string `json:"PauseResidency,omitempty"`
	PausedResidents   []string `json:"PausedResidents,omitempty"`
	ConfirmedResident []string `json:"ConfirmedResident,omitempty"`
	ConfirmedSet      []string `json:"ConfirmedSet,omitempty"`
	EngineResident    []string `json:"EngineResident,omitempty"`
	EngineResidency   []string `json:"EngineResidency,omitempty"`
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
}
