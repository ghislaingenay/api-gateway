package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"api-gateway/internal/gateway"

	"github.com/go-playground/validator/v10"
)

// RouteResolver resolves the static route for a method+path, giving access
// to its per-route BodySchema/RequiredParams. Declared here (the consumer)
// per the DI convention; *gateway.RouteTable satisfies it structurally.
type RouteResolver interface {
	Resolve(method, path string) (*gateway.Route, bool)
}

// ValidationMiddleware validates a request's body and required path/query
// parameters against the schema configured on its resolved route, before
// the request reaches rate limiting, caching, or routing. It must run after
// auth.JWTAuthMiddleware per FEAT-007's business rule (validation runs
// after authentication/authorization), though it does not itself read
// authenticated identity. A route with no BodySchema and no RequiredParams
// is passed through unvalidated — the gateway performs structural
// validation only where a schema is configured (FEAT-007 Non-Goals).
func ValidationMiddleware(routes RouteResolver, maxBodyBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, ok := gateway.RouteFromContext(r.Context())
			if !ok {
				route, ok = routes.Resolve(r.Method, r.URL.Path)
				if ok {
					r = r.WithContext(gateway.WithRoute(r.Context(), route))
				}
			}
			if !ok || (route.BodySchema == nil && len(route.RequiredParams) == 0) {
				next.ServeHTTP(w, r)
				return
			}

			var fieldErrors []FieldError
			fieldErrors = append(fieldErrors, validateParams(r, route)...)

			if route.BodySchema != nil {
				bodyErrors, done := validateBody(w, r, route.BodySchema, maxBodyBytes)
				if done {
					// A malformed/oversized body already wrote its own 400
					// response; stop here rather than also reporting field
					// errors collected above.
					return
				}
				fieldErrors = append(fieldErrors, bodyErrors...)
			}

			if len(fieldErrors) > 0 {
				writeValidationError(w, fieldErrors)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// validateParams checks every configured required path/query parameter,
// returning one FieldError per missing or type-mismatched value.
func validateParams(r *http.Request, route *gateway.Route) []FieldError {
	var errs []FieldError
	for _, p := range route.RequiredParams {
		var value string
		switch p.In {
		case gateway.ParamQuery:
			value = r.URL.Query().Get(p.Name)
		case gateway.ParamPath:
			value = pathParamValue(route.Path, r.URL.Path)
		}

		if value == "" {
			errs = append(errs, FieldError{Field: p.Name, Reason: "required"})
			continue
		}
		if err := validateVar(value, p.Rule); err != nil {
			errs = append(errs, FieldError{Field: p.Name, Reason: reasonFor(err)})
		}
	}
	return errs
}

// pathParamValue extracts the path segment matched by a route's trailing
// wildcard (e.g. pattern "/api/orders/*" against path "/api/orders/abc"
// yields "abc"). Returns "" if the pattern has no wildcard or the request
// path has no trailing segment.
func pathParamValue(pattern, path string) string {
	prefix, ok := strings.CutSuffix(pattern, "/*")
	if !ok {
		return ""
	}
	remainder := strings.TrimPrefix(path, prefix)
	return strings.Trim(remainder, "/")
}

// validateBody reads and restores the request body (so the downstream
// proxy still receives it), enforces the max body size, and validates it
// against the route's BodySchema. If the body is oversized or not valid
// JSON, it writes the 400 response itself and returns done=true so the
// caller stops processing immediately.
func validateBody(w http.ResponseWriter, r *http.Request, schema *gateway.BodySchema, maxBodyBytes int64) (fieldErrors []FieldError, done bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if closeErr := r.Body.Close(); closeErr != nil {
		log.Printf("validation: failed to close request body: %v", closeErr)
	}
	if err != nil {
		writeValidationMessage(w, "unable to read request body")
		return nil, true
	}
	if int64(len(raw)) > maxBodyBytes {
		writeValidationMessage(w, "request body exceeds maximum allowed size")
		return nil, true
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	if len(raw) == 0 {
		if schema.Required {
			return []FieldError{{Field: "body", Reason: "required"}}, false
		}
		return nil, false
	}

	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeValidationMessage(w, "request body must be valid JSON")
		return nil, true
	}

	var errs []FieldError
	for _, f := range schema.Fields {
		value, present := body[f.Field]
		if !present || value == nil {
			if strings.Contains(f.Rule, "required") {
				errs = append(errs, FieldError{Field: f.Field, Reason: "required"})
			}
			continue
		}
		if err := validateVar(value, f.Rule); err != nil {
			errs = append(errs, FieldError{Field: f.Field, Reason: reasonFor(err)})
		}
	}
	return errs, false
}

// reasonFor reduces a validator error down to its first failing tag (e.g.
// "required", "email", "uuid4") for the field-level error response.
func reasonFor(err error) string {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) && len(validationErrs) > 0 {
		return validationErrs[0].Tag()
	}
	return "invalid"
}

func writeValidationError(w http.ResponseWriter, fields []FieldError) {
	writeError(w, ErrorResponse{
		Error:   "validation_failed",
		Message: "request validation failed",
		Fields:  fields,
	})
}

func writeValidationMessage(w http.ResponseWriter, message string) {
	writeError(w, ErrorResponse{
		Error:   "validation_failed",
		Message: message,
		Fields:  []FieldError{},
	})
}

func writeError(w http.ResponseWriter, resp ErrorResponse) {
	if resp.Fields == nil {
		resp.Fields = []FieldError{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("validation: failed to write error response: %v", err)
	}
}
