// Package rules provides struct/value validation via go-playground/validator
// tags, including custom tags (slug, timezone) shared across the codebase.
// It has no dependency on gateway, so packages like auth that need generic
// validation can depend on it without risking an import cycle.
package rules

import (
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
)

var (
	instance *validator.Validate
	once     sync.Once
	initErr  error
)

// getValidator returns the singleton validator instance, initializing it once.
func getValidator() (*validator.Validate, error) {
	once.Do(func() {
		v := validator.New()

		if err := v.RegisterValidation("slug", validateSlug); err != nil {
			initErr = err
			return
		}

		if err := v.RegisterValidation("timezone", validateTimezone); err != nil {
			initErr = err
			return
		}

		instance = v
	})

	return instance, initErr
}

// validateSlug allows lowercase letters, digits, and hyphens, and must not
// start or end with a hyphen.
func validateSlug(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

// validateTimezone checks that the field is a valid IANA time zone name.
func validateTimezone(fl validator.FieldLevel) bool {
	_, err := time.LoadLocation(fl.Field().String())
	return err == nil
}

// Validate validates a struct's fields against its `validate` tags.
func Validate(s interface{}) error {
	validate, err := getValidator()
	if err != nil {
		return err
	}
	return validate.Struct(s)
}

// Var validates a single value against a validator tag string (e.g.
// "required,email"), used to check body fields and path/query parameters
// that aren't backed by a Go struct.
func Var(value interface{}, tag string) error {
	validate, err := getValidator()
	if err != nil {
		return err
	}
	return validate.Var(value, tag)
}
