package update

type Updater interface {
	Create(Record) error
	Delete(string) error
	//OnServiceUp(Record) error
	//OnServiceDown(Record) error
}
