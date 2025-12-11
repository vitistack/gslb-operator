package loaders

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/joho/godotenv"
)

type FileLoader struct {
	fileNames []string
}

func NewFileLoader(fileNames ...string) *FileLoader {
	return &FileLoader{
		fileNames: fileNames,
	}
}

func (f *FileLoader) Load(dest any) error {
	for _, file := range f.fileNames {
		var err error
		switch {
		case strings.HasSuffix(file, ".env"):
			err = f.loadDotEnv(dest, file)

		case strings.HasSuffix(file, ".json"):
			err = f.loadJSON(dest, file)

		default:
			err = f.loadDotEnv(dest, file)
		}
		if err != nil {
			return fmt.Errorf("could not load file: %s", file)
		}
	}
	return nil
}

func (f *FileLoader) loadDotEnv(dest any, file string) error {
	val := reflect.ValueOf(dest).Elem()
	typ := val.Type()

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("unable to load config file: %s: destination must be a struct pointer", file)
	}

	cfg, err := godotenv.Read(file)
	if err != nil {
		return fmt.Errorf("unable to read config file: %s: %s", file, err.Error())
	}

	for i := range val.NumField() {
		field := val.Field(i)
		fieldTyp := typ.Field(i)

		if !field.CanSet() {
			continue
		}

		tag, ok := fieldTyp.Tag.Lookup("env")
		if !ok {
			continue
		}

		envValue, ok := cfg[tag]
		if ok {
			if err := setEnvironmentVariable(field, envValue); err != nil {
				return fmt.Errorf("unable to load config: %s", err.Error())
			}
		}
	}

	return nil
}

func (f *FileLoader) loadJSON(dest any, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read config file: %s: %s", file, err.Error())
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to parse config file: %s", err.Error())
	}

	return nil
}
