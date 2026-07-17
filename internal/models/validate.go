package models

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()

	v.RegisterValidation("slug", validateSlug)
	v.RegisterValidation("timezone", validateTimezone)

	return v
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

// Validate runs struct-level validation on any of the identity models,
// checking the `validate` tags declared on their fields.
func Validate(s interface{}) error {
	return validate.Struct(s)
}
