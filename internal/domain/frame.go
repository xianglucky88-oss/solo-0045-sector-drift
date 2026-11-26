package domain

type Frame struct {
	Index      int
	Visit      Visit
	Required   []string
	Resident   []string
	Bytes      int64
	Operations []Op
	Decisions  []Decision

	// FlatOperations expands atomic group ops into per-member ops carrying the
	// same Group/Batch correlation. It mirrors Operations for ungrouped plans.
	FlatOperations  []Op `json:"FlatOperations,omitempty"`
	AssetOperations []Op `json:"AssetOperations,omitempty"`

	// GroupOperations contains only atomic resource-group ops in this frame.
	GroupOperations []Op `json:"GroupOperations,omitempty"`

	// Batches mirrors Operations when the frame touches resource groups,
	// giving engines an explicit atomic batch list.
	Batches []Op `json:"Batches,omitempty"`
	// BatchSets lists the member id sets that must be applied atomically.
	BatchSets [][]string `json:"BatchSets,omitempty"`

	// LoadedGroups / ResidentGroups name the groups resident after the frame.
	LoadedGroups   []string `json:"LoadedGroups,omitempty"`
	ResidentGroups []string `json:"ResidentGroups,omitempty"`
	// RequiredGroups names the groups pulled in by the current sector closure.
	RequiredGroups []string `json:"RequiredGroups,omitempty"`
}
