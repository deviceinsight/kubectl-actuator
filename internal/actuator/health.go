package actuator

import (
	"fmt"
	"net/url"
)

// GetHealth fetches /health, or /health/{group} when group names a health
// group or component.
func (c *actuatorClient) GetHealth(group string) (*HealthResponse, error) {
	body, err := c.getHealthBody(group)
	if err != nil {
		return nil, err
	}
	var healthResponse HealthResponse
	if err := parseJSON(body, &healthResponse); err != nil {
		return nil, c.notJSONError(err, "failed to get health")
	}
	return &healthResponse, nil
}

// GetHealthRaw is GetHealth for the structured output path: same status
// handling, but the endpoint's response is passed through verbatim.
func (c *actuatorClient) GetHealthRaw(group string) ([]byte, error) {
	return c.getHealthBody(group)
}

// getHealthBody fetches the health document. Spring serves the full document
// with HTTP 503 when the aggregate status is DOWN or OUT_OF_SERVICE, so a
// 503 carrying a health body is a successful check of an unhealthy
// application, not a failure.
func (c *actuatorClient) getHealthBody(group string) ([]byte, error) {
	path := "/health"
	if group != "" {
		path += "/" + url.PathEscape(group)
	}

	resp, err := c.httpClient.Get(path)
	if err != nil {
		return nil, c.connectionError(err)
	}
	if resp.StatusCode == 503 && looksLikeHealthBody(resp.Body) {
		return resp.Body, nil
	}
	if resp.IsErrorStatus() {
		if group != "" {
			return nil, c.resourceStatusError("health", resp.StatusCode, resp.Status, "failed to get health",
				fmt.Errorf("failed to get health: no health group or component named %q", group))
		}
		return nil, c.statusError("health", resp.StatusCode, resp.Status, "failed to get health")
	}
	return resp.Body, nil
}

// looksLikeHealthBody reports whether a 503 body is a health document rather
// than an error response from something that is not the actuator.
func looksLikeHealthBody(body []byte) bool {
	var probe struct {
		Status string `json:"status"`
	}
	return parseJSON(body, &probe) == nil && probe.Status != ""
}

type HealthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]HealthComponent `json:"components"`
	Groups     []string                   `json:"groups,omitempty"`
}

type HealthComponent struct {
	Status     string                     `json:"status"`
	Components map[string]HealthComponent `json:"components,omitempty"`
	Details    map[string]any             `json:"details,omitempty"`
}
