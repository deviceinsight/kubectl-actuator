package actuator

import (
	"strings"
)

type ActuatorIndex struct {
	Links map[string]Link `json:"_links"`
}

type Link struct {
	Href      string `json:"href"`
	Templated bool   `json:"templated"`
}

func (c *actuatorClient) GetActuatorIndex() (*ActuatorIndex, error) {
	var index ActuatorIndex
	if err := c.getAndParse("", "", "failed to get actuator index", &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func (c *actuatorClient) GetAvailableEndpoints() ([]string, error) {
	index, err := c.GetActuatorIndex()
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, len(index.Links))
	seen := make(map[string]bool)

	for name, link := range index.Links {
		if name == "self" {
			continue
		}

		// For templated endpoints, extract the base name
		// e.g., "health-path" -> "health", "loggers-name" -> "loggers"
		baseName := name
		if link.Templated {
			// Remove common suffixes from templated endpoints
			baseName = strings.TrimSuffix(name, "-path")
			baseName = strings.TrimSuffix(baseName, "-name")
			baseName = strings.TrimSuffix(baseName, "-cache")
			baseName = strings.TrimSuffix(baseName, "-prefix")
			baseName = strings.TrimSuffix(baseName, "-toMatch")
			baseName = strings.TrimSuffix(baseName, "-requiredMetricName")
		}

		if !seen[baseName] {
			seen[baseName] = true
			endpoints = append(endpoints, baseName)
		}
	}

	return endpoints, nil
}
