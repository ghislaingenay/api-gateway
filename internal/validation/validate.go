package validation

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


func Validate(s interface{}) error {
	validate, err := getValidator()
	if err != nil {
		return err
	}
	return validate.Struct(s)
}

