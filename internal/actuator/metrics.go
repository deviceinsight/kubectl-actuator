package actuator

import (
	"fmt"
	"net/url"
	"strings"
)

func (c *actuatorClient) GetMetrics() (*MetricsListResponse, error) {
	var metricsResponse MetricsListResponse
	if err := c.getAndParse("/metrics", "metrics", "failed to get metrics", &metricsResponse); err != nil {
		return nil, err
	}
	return &metricsResponse, nil
}

// MetricPath builds the /metrics/{name} path, with tag=key:value query
// parameters for drill-down when tags are given. Exported for the metrics
// command's raw passthrough, which fetches the same path via GetRaw.
func MetricPath(metricName string, tags []string) string {
	path := "/metrics/" + url.PathEscape(metricName)
	if len(tags) > 0 {
		query := url.Values{}
		for _, tag := range tags {
			query.Add("tag", tag)
		}
		path += "?" + query.Encode()
	}
	return path
}

func (c *actuatorClient) GetMetric(metricName string, tags []string) (*MetricResponse, error) {
	resp, err := c.httpClient.Get(MetricPath(metricName, tags))
	if err != nil {
		return nil, c.connectionError(err)
	}

	if resp.IsErrorStatus() {
		if resp.StatusCode == 400 && len(tags) > 0 {
			return nil, fmt.Errorf("failed to get metric %q with tags %s: %s", metricName, strings.Join(tags, ", "), resp.Status)
		}
		return nil, c.resourceStatusError("metrics", resp.StatusCode, resp.Status, "failed to get metric",
			resourceNotFoundError("metric", metricName, resp.Status))
	}

	var metricResponse MetricResponse
	if err := parseJSON(resp.Body, &metricResponse); err != nil {
		return nil, c.notJSONError(err, "failed to get metric")
	}

	return &metricResponse, nil
}

type MetricsListResponse struct {
	Names []string `json:"names"`
}

type MetricResponse struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	BaseUnit      string         `json:"baseUnit"`
	Measurements  []Measurement  `json:"measurements"`
	AvailableTags []AvailableTag `json:"availableTags"`
}

type Measurement struct {
	Statistic string  `json:"statistic"`
	Value     float64 `json:"value"`
}

type AvailableTag struct {
	Tag    string   `json:"tag"`
	Values []string `json:"values"`
}
