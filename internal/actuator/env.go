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
		return nil, err
	}

	if resp.IsErrorStatus() {
		if resp.StatusCode == 404 && c.isEndpointAccessible("/env") {
			return nil, resourceNotFoundError("property", propertyName, resp.Status)
		}
		return nil, endpointError("env", resp.Status, "failed to get property")
	}

	var propertyResponse EnvPropertyResponse
	if err := parseJSON(resp.Body, &propertyResponse); err != nil {
		return nil, err
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
	Value  interface{} `json:"value"`
	Origin string      `json:"origin,omitempty"`
}

type EnvPropertyResponse struct {
	Property        PropertyValue             `json:"property"`
	ActiveProfiles  []string                  `json:"activeProfiles"`
	DefaultProfiles []string                  `json:"defaultProfiles"`
	PropertySources []PropertySourceReference `json:"propertySources"`
}

type PropertyValue struct {
	Source string      `json:"source"`
	Value  interface{} `json:"value"`
}

type PropertySourceReference struct {
	Name     string      `json:"name"`
	Property interface{} `json:"property,omitempty"`
}
