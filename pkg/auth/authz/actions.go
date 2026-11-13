package authz

// Action is the logical, transport-independent authorization key for a route.
type Action string

const (
	ServicesList Action = "services.list"
	ServicesRead Action = "services.read"

	SpoofsList Action = "spoofs.list"
	SpoofsRead Action = "spoofs.read"

	OverrideList   Action = "override.list"
	OverrideRead   Action = "override.read"
	OverrideCreate Action = "override.create"
	OverrideDelete Action = "override.Delete"

	StatusList Action = "status.list"
	StatusRead Action = "status.read"

	MembersList Action = "members.list"
	MembersRead Action = "members.read"
)

// KnownActions is the full action inventory the operator understands; a
// policy granting anything outside this set is rejected rather than compiled.
var KnownActions = []Action{
	ServicesList,
	ServicesRead,
	SpoofsList,
	SpoofsRead,
	OverrideList,
	OverrideRead,
	OverrideCreate,
	OverrideDelete,
	StatusList,
	StatusRead,
	MembersList,
	MembersRead,
}
