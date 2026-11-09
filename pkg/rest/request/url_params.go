package request

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

// Populates a struct object containing exported data members with the `param:""` tag set,
// with all the values that the urlValues contains. Be mindfull to only pass the query string, and not anything host related.
func UnmarshalParams[T any](urlValues url.Values, dest *T) error {
	val := reflect.ValueOf(dest).Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("cannot marshall params, destination is not a struct")
	}

	knownParams := make(map[string]struct{})
	if err := unmarshalStruct(val, urlValues, knownParams); err != nil {
		return err
	}

	// reject query params that have no matching `param` tag on dest
	for key := range urlValues {
		if _, ok := knownParams[key]; !ok {
			return fmt.Errorf("unknown url param: %s", key)
		}
	}

	return nil
}

func unmarshalStruct(val reflect.Value, urlValues url.Values, knownParams map[string]struct{}) error {
	valType := val.Type()
	for i := range val.NumField() {
		field := val.Field(i)
		fieldType := valType.Field(i)

		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			if err := unmarshalStruct(field, urlValues, knownParams); err != nil {
				return err
			}
			continue
		}

		tag, ok := fieldType.Tag.Lookup("param")
		if !ok {
			continue
		}
		knownParams[tag] = struct{}{}

		if value := urlValues.Get(tag); value != "" {
			if err := setField(field, value); err != nil {
				return fmt.Errorf("unable to marshall url params: %s", err.Error())
			}
		}
	}

	return nil
}

func MarshallParams[T any](params *T) url.Values {
	values := make(url.Values)
	val := reflect.ValueOf(params).Elem()
	valType := val.Type()

	if valType.Kind() != reflect.Struct {
		return values
	}

	numFields := val.NumField()
	for i := range numFields {
		field := val.Field(i)
		fieldType := valType.Field(i)

		tag, ok := fieldType.Tag.Lookup("param")
		if ok {
			val, ok := field.Interface().(string)
			if ok {
				values.Add(tag, val)
			}
		}
	}

	return values
}

// populates the field with the value of value
func setField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return nil
	}

	if field.Kind() == reflect.Pointer {
		ptr := reflect.New(field.Type().Elem())
		if err := setField(ptr.Elem(), value); err != nil {
			return err
		}
		field.Set(ptr)
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		num, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to convert value: %s", err.Error())
		}
		field.SetInt(num)
	case reflect.Bool:
		boolean, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("unable to convert value: %s", err.Error())
		}
		field.SetBool(boolean)
	}

	return nil
}
