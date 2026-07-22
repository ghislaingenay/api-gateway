package health

// HealthResponse is the GET /health liveness response body (FEAT-009 FR-2).
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse is the GET /ready readiness response body (FEAT-009 FR-3).
type ReadyResponse struct {
	Status   string `json:"status"`
	Redis    string `json:"redis"`
	Postgres string `json:"postgres"`
}
