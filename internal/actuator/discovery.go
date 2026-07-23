package actuator

import (
	"fmt"
	"strings"
)

// actuatorIndex is the discovery document served at the actuator base path.
type actuatorIndex struct {
	Links map[string]indexLink `json:"_links"`
}

type indexLink struct {
	Href      string `json:"href"`
	Templated bool   `json:"templated"`
}

// templatedLinkSuffixes are the template-variable names Spring appends to a
// templated link's id: "health-path" -> "health", "loggers-name" -> "loggers".
var templatedLinkSuffixes = []string{
	"-path", "-name", "-cache", "-prefix", "-toMatch", "-requiredMetricName",
}

// getActuatorIndex fetches the discovery document. Index failures are
// reported plainly: the index is itself the probe the 404 diagnosis relies
// on, so it must not recurse into diagnosis.
func (c *actuatorClient) getActuatorIndex() (*actuatorIndex, error) {
	resp, err := c.httpClient.Get("")
	if err != nil {
		return nil, c.connectionError(err)
	}
	if resp.IsErrorStatus() {
		return nil, fmt.Errorf("failed to get actuator index: %s", resp.Status)
	}
	var index actuatorIndex
	if err := parseJSON(resp.Body, &index); err != nil {
		return nil, c.notJSONError(err, "failed to get actuator index")
	}
	return &index, nil
}

func (c *actuatorClient) GetAvailableEndpoints() ([]string, error) {
	index, err := c.getActuatorIndex()
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, len(index.Links))
	seen := make(map[string]bool)

	for name, link := range index.Links {
		if name == "self" {
			continue
		}

		baseName := name
		if link.Templated {
			for _, suffix := range templatedLinkSuffixes {
				baseName = strings.TrimSuffix(baseName, suffix)
			}
		}

		if !seen[baseName] {
			seen[baseName] = true
			endpoints = append(endpoints, baseName)
		}
	}

	return endpoints, nil
}
