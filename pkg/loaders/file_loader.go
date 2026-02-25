package loaders

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/joho/godotenv"
)

type FileLoader struct {
	fileNames []string
}

func NewFileLoader(fileNames ...string) (*FileLoader, error) {
	loader := &FileLoader{
		fileNames: make([]string, 0, len(fileNames)),
	}
	for _, file := range fileNames {
		info, err := os.Stat(file)
		if err == nil { // silently drop files that dont exist
			if info.IsDir() {
				err := filepath.Walk(file, func(path string, info fs.FileInfo, err error) error {
					if err != nil {
						return err
					}

					if info.IsDir() {
						return nil
					}

					loader.fileNames = append(loader.fileNames, path)
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("could not list files in directory: %w", err)
				}
			} else {
				loader.fileNames = append(loader.fileNames, file)
			}
		}
	}

	return loader, nil
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
			err = f.loadPlainText(dest, file)
		}
		if err != nil {
			return fmt.Errorf("could not load file: %s: %w", file, err)
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
				return fmt.Errorf("unable to set environment variable: %s", err.Error())
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

func (f *FileLoader) loadPlainText(dest any, file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("unable to load file: %s: %w", file, err)
	}

	if info.IsDir() { // skip directories
		return nil
	}

	val := reflect.ValueOf(dest).Elem()
	typ := val.Type()

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("unable to load config file: %s: destination must be a struct pointer", file)
	}
	rawData, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read file: %s: %w", file, err)
	}
	data := string(rawData)

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

		if strings.Contains(file, tag) { // file name must contain the struct tag
			if err := setEnvironmentVariable(field, data); err != nil {
				return fmt.Errorf("unable to set struct value: %w", err)
			}
		}
	}

	return nil
}
