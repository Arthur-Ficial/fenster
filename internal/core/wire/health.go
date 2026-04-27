package wire

// HealthInfo is the /health response envelope.
type HealthInfo struct {
	Status             string   `json:"status"`
	Model              string   `json:"model"`
	Version            string   `json:"version"`
	ActiveRequests     int      `json:"active_requests"`
	ContextWindow      int      `json:"context_window"`
	ModelAvailable     bool     `json:"model_available"`
	SupportedLanguages []string `json:"supported_languages"`
}

// ModelObject is one entry in /v1/models.
type ModelObject struct {
	ID                    string   `json:"id"`
	Object                string   `json:"object"`
	Created               int64    `json:"created"`
	OwnedBy               string   `json:"owned_by"`
	ContextWindow         int      `json:"context_window"`
	SupportedParameters   []string `json:"supported_parameters"`
	UnsupportedParameters []string `json:"unsupported_parameters"`
	Notes                 string   `json:"notes,omitempty"`
}

// ModelsList is the /v1/models response envelope.
type ModelsList struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}
