package validation

// FieldError reports a single field or parameter that failed validation.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ErrorResponse is the consistent JSON shape returned for every validation
// failure across all routes (FEAT-007 FR-3).
type ErrorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields"`
}
