package rules

// QType represents a DNS query type.
type QType uint16

const (
	QTypeA     QType = 1
	QTypeNS    QType = 2
	QTypeCNAME QType = 5
	QTypeSOA   QType = 6
	QTypePTR   QType = 12
	QTypeTXT   QType = 16
	QTypeMX    QType = 15
	QTypeAAAA  QType = 28
	QTypeSRV   QType = 33
	QTypeANY   QType = 255
)

// holds the field values for a dnsdist rule
type RuleLine struct {
	ID      string
	Name    string
	UUID    string
	Matches string
	Rule    string
	Action  string
}

type Rule interface {
	luaRule() string
}

type Action interface {
	luaAction() string
}

type Table interface {
	luaTable() string
}

