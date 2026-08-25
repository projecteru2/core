package utils

import (
	"os"
	"reflect"

	"github.com/cockroachdb/errors"
	"gopkg.in/yaml.v3"

	"github.com/projecteru2/core/types"
)

type fieldWalker func(reflect.Value) error

// LoadConfig loads the config from the YAML file at configPath.
func LoadConfig(configPath string) (types.Config, error) {
	config := types.Config{}
	value := reflect.ValueOf(&config).Elem()
	if err := applyDefaults(value); err != nil {
		return config, err
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // the config path comes from the operator's own flag
	if err != nil {
		return config, err
	}
	if err = yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, checkRequired(value)
}

// defaults land before the file is read so an explicit zero in the file still wins
func applyDefaults(value reflect.Value) error {
	for i := range value.NumField() {
		field := value.Field(i)
		if !field.CanSet() {
			continue
		}
		structField := value.Type().Field(i)
		if tag := structField.Tag.Get("default"); tag != "" && field.IsZero() {
			if err := yaml.Unmarshal([]byte(tag), field.Addr().Interface()); err != nil {
				return errors.Wrapf(err, "bad default for %s", structField.Name)
			}
		}
		if err := walkNested(field, applyDefaults); err != nil {
			return err
		}
	}
	return nil
}

func checkRequired(value reflect.Value) error {
	for i := range value.NumField() {
		field := value.Field(i)
		if !field.CanSet() {
			continue
		}
		if structField := value.Type().Field(i); structField.Tag.Get("required") == "true" && field.IsZero() {
			return errors.Newf("%s is required, but blank", structField.Name)
		}
		if err := walkNested(field, checkRequired); err != nil {
			return err
		}
	}
	return nil
}

func walkNested(field reflect.Value, walk fieldWalker) error {
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.Struct:
		return walk(field)
	case reflect.Slice:
		for i := range field.Len() {
			if elem := reflect.Indirect(field.Index(i)); elem.Kind() == reflect.Struct {
				if err := walk(elem); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
