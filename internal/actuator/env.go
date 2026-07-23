package actuator

import "net/url"

func (c *actuatorClient) GetEnv() (*EnvResponse, error) {
	var envResponse EnvResponse
	if err := c.getAndParse("/env", "env", "failed to get environment", &envResponse); err != nil {
		return nil, err
	}
	return &envResponse, nil
}

func (c *actuatorClient) GetEnvProperty(propertyName string) (*EnvPropertyResponse, error) {
	path := "/env/" + url.PathEscape(propertyName)
	resp, err := c.httpClient.Get(path)
	if err != nil {
		return nil, c.connectionError(err)
	}

	if resp.IsErrorStatus() {
		return nil, c.resourceStatusError("env", resp.StatusCode, resp.Status, "failed to get property",
			resourceNotFoundError("property", propertyName, resp.Status))
	}

	var propertyResponse EnvPropertyResponse
	if err := parseJSON(resp.Body, &propertyResponse); err != nil {
		return nil, c.notJSONError(err, "failed to get property")
	}

	return &propertyResponse, nil
}

type EnvResponse struct {
	ActiveProfiles  []string         `json:"activeProfiles"`
	PropertySources []PropertySource `json:"propertySources"`
}

type PropertySource struct {
	Name       string                     `json:"name"`
	Properties map[string]PropertyDetails `json:"properties"`
}

type PropertyDetails struct {
	Value  any    `json:"value"`
	Origin string `json:"origin,omitempty"`
}

type EnvPropertyResponse struct {
	Property        PropertyValue             `json:"property"`
	ActiveProfiles  []string                  `json:"activeProfiles"`
	DefaultProfiles []string                  `json:"defaultProfiles"`
	PropertySources []PropertySourceReference `json:"propertySources"`
}

type PropertyValue struct {
	Source string `json:"source"`
	Value  any    `json:"value"`
}

type PropertySourceReference struct {
	Name     string `json:"name"`
	Property any    `json:"property,omitempty"`
}
