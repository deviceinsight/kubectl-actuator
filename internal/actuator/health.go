package actuator

func (c *actuatorClient) GetHealth() (*HealthResponse, error) {
	var healthResponse HealthResponse
	if err := c.getAndParse("/health", "health", "failed to get health", &healthResponse); err != nil {
		return nil, err
	}
	return &healthResponse, nil
}

type HealthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]HealthComponent `json:"components"`
	Groups     []string                   `json:"groups,omitempty"`
}

type HealthComponent struct {
	Status     string                     `json:"status"`
	Components map[string]HealthComponent `json:"components,omitempty"`
	Details    map[string]interface{}     `json:"details,omitempty"`
}
