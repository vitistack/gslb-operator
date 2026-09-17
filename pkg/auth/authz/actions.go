package authz

// Action is the logical, transport-independent authorization key for a route.
type Action string

const (
	SpoofsList Action = "spoofs.list"
	SpoofsRead Action = "spoofs.read"

	OverrideList   Action = "override.list"
	OverrideRead   Action = "override.read"
	OverrideCreate Action = "override.create"
	OverrideDelete Action = "override.Delete"

	StatusList Action = "status.list"
	StatusRead Action = "status.read"
)
