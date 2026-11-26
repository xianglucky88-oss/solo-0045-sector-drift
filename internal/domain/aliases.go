package domain

// Vocabulary aliases give Go callers the same broad domain names accepted on
// the JSON boundary without duplicating behavior.
type (
	Show               = Schedule
	ShowSchedule       = Schedule
	Performance        = Schedule
	Production         = Schedule
	FramingSchedule    = Schedule
	WarmupConfig       = Schedule
	ScheduleConfig     = Schedule
	Shot               = Cue
	PhotoOp            = Cue
	DoorShot           = Cue
	CameraCue          = Cue
	FramingOpportunity = Cue
	Window             = FrameWindow
	GateWindow         = FrameWindow
	OpenWindow         = FrameWindow
	Warmup             = Booking
	Setup              = Booking
	Fulfillment        = Booking
)
