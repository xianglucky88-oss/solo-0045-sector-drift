package domain

type Request struct {
	Sectors []Sector
	Assets  []Asset
	Gates   []Gate
	Route   []Visit
	Policy  Policy

	// Lease declarations. Leases is the canonical name; the remaining fields
	// are wire aliases accepted from documents that name the same concept
	// differently. leaseList tolerates a single object as well as an array.
	Leases            leaseList `json:"Leases,omitempty"`
	Lease             leaseList `json:"Lease,omitempty"`
	LeaseDeclarations leaseList `json:"LeaseDeclarations,omitempty"`
	LeaseDeclaration  leaseList `json:"LeaseDeclaration,omitempty"`
	LeaseDecls        leaseList `json:"LeaseDecls,omitempty"`
	Tenures           leaseList `json:"Tenures,omitempty"`
	Holds             leaseList `json:"Holds,omitempty"`
	ResourceHolds     leaseList `json:"ResourceHolds,omitempty"`
	Vouchers          leaseList `json:"Vouchers,omitempty"`
	Credentials       leaseList `json:"Credentials,omitempty"`
	Permits           leaseList `json:"Permits,omitempty"`
	Tickets           leaseList `json:"Tickets,omitempty"`
	Cutscenes         leaseList `json:"Cutscenes,omitempty"`
	References        leaseList `json:"References,omitempty"`
	Refs              leaseList `json:"Refs,omitempty"`
	Continuations     leaseList `json:"Continuations,omitempty"`
	StreamLeases      leaseList `json:"StreamLeases,omitempty"`
	ResourceLeases    leaseList `json:"ResourceLeases,omitempty"`
	LeasedResources   leaseList `json:"LeasedResources,omitempty"`
	Retainers         leaseList `json:"Retainers,omitempty"`
	Pins              leaseList `json:"Pins,omitempty"`

	// Optional runtime event timeline supplied directly to the build entry.
	// When present a replay report is attached to the produced artifact.
	Events      []LeaseEvent   `json:"Events,omitempty"`
	EventLog    []LeaseEvent   `json:"EventLog,omitempty"`
	Timeline    []LeaseEvent   `json:"Timeline,omitempty"`
	Trace       []LeaseEvent   `json:"Trace,omitempty"`
	Replay      *ReplayRequest `json:"Replay,omitempty"`
	ReplayInput *ReplayRequest `json:"ReplayInput,omitempty"`
	Drill       *ReplayRequest `json:"Drill,omitempty"`
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

	// Leases may alternatively be nested under Policy.
	Leases         leaseList `json:"Leases,omitempty"`
	Holds          leaseList `json:"Holds,omitempty"`
	LeaseBudget    int64     `json:"LeaseBudget,omitempty"`
	ResidentLeases leaseList `json:"ResidentLeases,omitempty"`
}
