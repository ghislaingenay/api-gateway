package onboarding

// OnboardRequest is the body of POST /onboarding.
type OnboardRequest struct {
	OrganizationName string `json:"organization_name" validate:"required,min=2,max=255"`
}

// OnboardResponse is returned on successful onboarding.
type OnboardResponse struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
}
