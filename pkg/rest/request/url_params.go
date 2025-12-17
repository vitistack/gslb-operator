package request

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

// Populates a struct object containing exported data members with the `param:""` tag set,
// with all the values that the urlValues contains. Be mindfull to only pass the query string, and not anything host related.
func MarshallParams[T any](urlValues url.Values, dest *T) error {
	val := reflect.ValueOf(dest).Elem()
	valType := val.Type()

	if valType.Kind() != reflect.Struct {
		return fmt.Errorf("cannot marshall params, destination is not a struct")
	}

	numFields := val.NumField()
	for i := range numFields {
		field := val.Field(i)
		fieldType := valType.Field(i)

		tag, tagOK := fieldType.Tag.Lookup("param")
		value := ""
		if urlValues.Has(tag) {
			value = urlValues.Get(tag)
		}

		if tagOK && value != "" {
			err := setField(field, value)
			if err != nil {
				return fmt.Errorf("unable to marshall url params: %s", err.Error())
			}
		}
	}

	return nil
}

func UnMarshallParams[T any](params *T) url.Values {
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
