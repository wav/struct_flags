package struct_flags

import (
	"gopkg.in/go-playground/validator.v9"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var Validator = validator.New()

func init() {
	if err := Validator.RegisterValidation("file", validateFile); err != nil {
		panic(err)
	}
	if err := Validator.RegisterValidation("resource_path", validateResourcePath); err != nil {
		panic(err)
	}
	if err := Validator.RegisterValidation("target_path", validateTargetPath); err != nil {
		panic(err)
	}
}

func defaultPrepareStructFields(arg interface{}) error {
	if arg != nil && reflect.Indirect(reflect.ValueOf(arg)).Kind() == reflect.Struct {
		return Validator.Struct(arg)
	}
	return nil
}

const (
	absoluteValidateFlag  = "absolute"
	existsValidateFlag    = "exists"
	notExistsValidateFlag = "not_exists"
)

func validateFile(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if path == "" {
		return true
	}
	for _, f := range strings.Split(strings.Trim(fl.Param(), ")"), ",") {
		switch f {
		case absoluteValidateFlag:
			if !filepath.IsAbs(path) {
				return false
			}
		case existsValidateFlag:
			if _, err := os.Stat(path); err != nil {
				println(err.Error())
				return false
			}
		case notExistsValidateFlag:
			if _, err := os.Stat(path); err == nil {
				return false
			}
		}
	}
	return true
}

var validateResourcePathPattern = regexp.MustCompile(`^[^/]{3,}(/[^/]+)*$`)

func validateResourcePath(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if path == "" {
		return true
	}
	return validateResourcePathPattern.MatchString(path)
}

var validateTargetPathPattern = regexp.MustCompile(`^[^/]{1,}(/[^/]+)*$`)

func validateTargetPath(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if path == "" {
		return true
	}
	return validateTargetPathPattern.MatchString(path)
}
