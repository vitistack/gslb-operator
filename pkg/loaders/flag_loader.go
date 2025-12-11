package loaders


type FlagLoader struct {}

func NewFlagLoader() *FileLoader {
	return &FileLoader{}

}

func (f *FlagLoader) Load(dest any) error {
	return nil
}