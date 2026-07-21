package gateway

// ParamLocation identifies where a route's required parameter is read from.
type ParamLocation string

const (
	ParamQuery ParamLocation = "query"
	ParamPath  ParamLocation = "path"
)

// FieldRule validates a single JSON body field using a go-playground
// validator tag string — the same syntax used by struct `validate:"..."`
// tags elsewhere in the project's data model.
type FieldRule struct {
	// Field is the JSON key to look up in the request body.
	Field string
	// Rule is a validator tag string, e.g. "required,email" or "gt=0".
	Rule string
}

// ParamRule validates a single required path or query parameter.
type ParamRule struct {
	// Name is the query parameter name (In: ParamQuery), or a label used
	// only for error reporting (In: ParamPath).
	Name string
	In   ParamLocation
	// Rule is a validator tag string, e.g. "required,uuid4".
	Rule string
}

// BodySchema is a route's per-request-body validation configuration
// (FEAT-007), applied by validation.ValidationMiddleware.
type BodySchema struct {
	// Required rejects an empty body with a 400.
	Required bool
	Fields   []FieldRule
}
