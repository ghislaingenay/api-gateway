package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRouteFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {})

	server := httptest.NewServer(withRouteFallback(mux))
	defer server.Close()

	t.Run("undefined route returns 404", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/auth/does-not-exist")
		if err != nil {
			t.Fatalf("error making request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404; got %v", resp.Status)
		}
		expectBody(t, resp, `{"error":"not_found","message":"no matching route"}`+"\n")
	})

	t.Run("defined route with wrong method returns 405", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/auth/login", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("error making request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405; got %v", resp.Status)
		}
		if resp.Header.Get("Allow") == "" {
			t.Errorf("expected Allow header to be set")
		}
		expectBody(t, resp, `{"error":"method_not_allowed","message":"method not allowed for this route"}`+"\n")
	})
}

func expectBody(t *testing.T, resp *http.Response, expected string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body: %v", err)
	}
	if string(body) != expected {
		t.Errorf("expected response body %q; got %q", expected, string(body))
	}
}
