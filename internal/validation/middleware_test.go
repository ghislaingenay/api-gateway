package validation

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api-gateway/internal/gateway"
)

type staticRouteResolver struct {
	route *gateway.Route
	ok    bool
}

func (s *staticRouteResolver) Resolve(method, path string) (*gateway.Route, bool) {
	return s.route, s.ok
}

func newNextRecorder() (http.Handler, *bool) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Body != nil {
			// Downstream (the proxy) must still be able to read the body.
			if _, err := io.ReadAll(r.Body); err != nil {
				panic(err)
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	return handler, &called
}

func decodeErrorResponse(t *testing.T, body *bytes.Buffer) ErrorResponse {
	t.Helper()
	var resp ErrorResponse
	if err := json.Unmarshal(body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v, body=%s", err, body.String())
	}
	return resp
}

func TestValidationMiddleware_NoSchema_PassesThrough(t *testing.T) {
	t.Parallel()
	routes := &staticRouteResolver{route: &gateway.Route{Path: "/api/orders"}, ok: true}
	next, called := newNextRecorder()

	handler := ValidationMiddleware(routes, 1<<20)(next)
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !*called {
		t.Fatal("expected next handler to be called when route has no schema")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestValidationMiddleware_NoRouteMatch_PassesThrough(t *testing.T) {
	t.Parallel()
	routes := &staticRouteResolver{ok: false}
	next, called := newNextRecorder()

	handler := ValidationMiddleware(routes, 1<<20)(next)
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !*called {
		t.Fatal("expected next handler to be called when no route matches")
	}
}

func TestValidationMiddleware_Body(t *testing.T) {
	t.Parallel()

	route := &gateway.Route{
		Path: "/api/orders",
		BodySchema: &gateway.BodySchema{
			Required: true,
			Fields: []gateway.FieldRule{
				{Field: "customer_email", Rule: "required,email"},
				{Field: "quantity", Rule: "required,gt=0"},
				{Field: "notes", Rule: "max=500"},
			},
		},
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCalled bool
		wantFields []string
	}{
		{
			name:       "valid body proceeds",
			body:       `{"customer_email":"a@example.com","quantity":2}`,
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "missing required field returns 400 with field details",
			body:       `{"quantity":2}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
			wantFields: []string{"customer_email"},
		},
		{
			name:       "invalid field value returns 400 with field details",
			body:       `{"customer_email":"not-an-email","quantity":2}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
			wantFields: []string{"customer_email"},
		},
		{
			name:       "type-mismatched numeric field returns 400",
			body:       `{"customer_email":"a@example.com","quantity":0}`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
			wantFields: []string{"quantity"},
		},
		{
			name:       "malformed non-JSON body returns 400 not 500",
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
		},
		{
			name:       "empty body when required returns 400",
			body:       ``,
			wantStatus: http.StatusBadRequest,
			wantCalled: false,
			wantFields: []string{"body"},
		},
		{
			name:       "unicode field values are accepted",
			body:       `{"customer_email":"a@example.com","quantity":1,"notes":"héllo wörld 日本語"}`,
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			routes := &staticRouteResolver{route: route, ok: true}
			next, called := newNextRecorder()
			handler := ValidationMiddleware(routes, 1<<20)(next)

			req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body=%s)", tt.wantStatus, w.Code, w.Body.String())
			}
			if *called != tt.wantCalled {
				t.Fatalf("expected next called=%v, got %v", tt.wantCalled, *called)
			}
			if tt.wantStatus == http.StatusBadRequest {
				resp := decodeErrorResponse(t, w.Body)
				if resp.Error != "validation_failed" {
					t.Fatalf("expected error code validation_failed, got %q", resp.Error)
				}
				for _, wantField := range tt.wantFields {
					found := false
					for _, f := range resp.Fields {
						if f.Field == wantField {
							found = true
						}
					}
					if !found {
						t.Fatalf("expected field error for %q, got %+v", wantField, resp.Fields)
					}
				}
			}
		})
	}
}

func TestValidationMiddleware_BodyRestoredForDownstream(t *testing.T) {
	t.Parallel()
	route := &gateway.Route{
		Path: "/api/orders",
		BodySchema: &gateway.BodySchema{
			Fields: []gateway.FieldRule{{Field: "quantity", Rule: "gt=0"}},
		},
	}
	routes := &staticRouteResolver{route: route, ok: true}

	var gotBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	})

	handler := ValidationMiddleware(routes, 1<<20)(next)
	body := `{"quantity":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotBody != body {
		t.Fatalf("expected downstream handler to receive original body %q, got %q", body, gotBody)
	}
}

func TestValidationMiddleware_OversizedBody(t *testing.T) {
	t.Parallel()
	route := &gateway.Route{
		Path:       "/api/orders",
		BodySchema: &gateway.BodySchema{Required: true},
	}
	routes := &staticRouteResolver{route: route, ok: true}
	next, called := newNextRecorder()

	handler := ValidationMiddleware(routes, 4)(next)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(`{"a":1}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
	if *called {
		t.Fatal("expected next not to be called for oversized body")
	}
}

func TestValidationMiddleware_RequiredParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		route      *gateway.Route
		target     string
		wantStatus int
		wantFields []string
	}{
		{
			name: "missing required query param returns 400",
			route: &gateway.Route{
				Path: "/api/orders",
				RequiredParams: []gateway.ParamRule{
					{Name: "status", In: gateway.ParamQuery, Rule: "required"},
				},
			},
			target:     "/api/orders",
			wantStatus: http.StatusBadRequest,
			wantFields: []string{"status"},
		},
		{
			name: "present query param proceeds",
			route: &gateway.Route{
				Path: "/api/orders",
				RequiredParams: []gateway.ParamRule{
					{Name: "status", In: gateway.ParamQuery, Rule: "required"},
				},
			},
			target:     "/api/orders?status=open",
			wantStatus: http.StatusOK,
		},
		{
			name: "type-mismatched path param (non-UUID) returns 400",
			route: &gateway.Route{
				Path: "/api/orders/*",
				RequiredParams: []gateway.ParamRule{
					{Name: "id", In: gateway.ParamPath, Rule: "required,uuid4"},
				},
			},
			target:     "/api/orders/not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantFields: []string{"id"},
		},
		{
			name: "valid UUID path param proceeds",
			route: &gateway.Route{
				Path: "/api/orders/*",
				RequiredParams: []gateway.ParamRule{
					{Name: "id", In: gateway.ParamPath, Rule: "required,uuid4"},
				},
			},
			target:     "/api/orders/dc9e0a2e-2c53-4c2a-9e40-4d3e8e3f5b1a",
			wantStatus: http.StatusOK,
		},
		{
			name: "missing path param segment returns 400",
			route: &gateway.Route{
				Path: "/api/orders/*",
				RequiredParams: []gateway.ParamRule{
					{Name: "id", In: gateway.ParamPath, Rule: "required,uuid4"},
				},
			},
			target:     "/api/orders",
			wantStatus: http.StatusBadRequest,
			wantFields: []string{"id"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			routes := &staticRouteResolver{route: tt.route, ok: true}
			next, called := newNextRecorder()
			handler := ValidationMiddleware(routes, 1<<20)(next)

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (body=%s)", tt.wantStatus, w.Code, w.Body.String())
			}
			wantCalled := tt.wantStatus == http.StatusOK
			if *called != wantCalled {
				t.Fatalf("expected next called=%v, got %v", wantCalled, *called)
			}
			if tt.wantStatus == http.StatusBadRequest {
				resp := decodeErrorResponse(t, w.Body)
				for _, wantField := range tt.wantFields {
					found := false
					for _, f := range resp.Fields {
						if f.Field == wantField {
							found = true
						}
					}
					if !found {
						t.Fatalf("expected field error for %q, got %+v", wantField, resp.Fields)
					}
				}
			}
		})
	}
}
