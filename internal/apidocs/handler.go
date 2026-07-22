// Package apidocs serves the gateway's OpenAPI 3.0 spec and an interactive
// Swagger UI at /docs (FEAT-010). It has no dependency on any other domain
// package: the spec is a static, embedded file.
package apidocs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

const swaggerUIPage = `<!DOCTYPE html>
<html>
  <head>
    <title>API Gateway - Swagger UI</title>
    <meta charset="utf-8" />
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.onload = () => {
        window.ui = SwaggerUIBundle({
          url: "/docs/openapi.yaml",
          dom_id: "#swagger-ui",
        });
      };
    </script>
  </body>
</html>
`

// SwaggerUIHandler returns an http.HandlerFunc for GET /docs: an HTML page
// that loads Swagger UI against the spec served at /docs/openapi.yaml.
func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerUIPage))
	}
}

// OpenAPISpecHandler returns an http.HandlerFunc for GET /docs/openapi.yaml:
// the raw embedded OpenAPI 3.0 spec.
func OpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openAPISpec)
	}
}
