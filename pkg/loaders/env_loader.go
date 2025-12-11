package loaders

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type EnvLoader struct{}

func NewEnvloader() *EnvLoader {
	return &EnvLoader{}
}

func (e *EnvLoader) Load(dest any) error {
	val := reflect.ValueOf(dest).Elem()
	typ := val.Type()

	kind := typ.Kind()

	if kind != reflect.Struct {
		return fmt.Errorf("unable to load config into destination: destination must be a struct pointer")
	}

	for i := range val.NumField() {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanSet() { // skip all fields that cannot be set
			continue
		}

		tag, ok := fieldType.Tag.Lookup("env")
		if !ok {
			continue
		}

		envValue, ok := os.LookupEnv(tag)
		if ok {
			if err := setEnvironmentVariable(field, envValue); err != nil {
				return fmt.Errorf("unable to load config: %s", err.Error())
			}
		}
	}

	return nil
}

func setEnvironmentVariable(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(num)
	case reflect.Bool:
		boolean, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolean)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			field.Set(reflect.ValueOf(values))
		}
	}
	return nil
}
