package actuator

import "net/url"

func (c *actuatorClient) GetMetrics() (*MetricsListResponse, error) {
	var metricsResponse MetricsListResponse
	if err := c.getAndParse("/metrics", "metrics", "failed to get metrics", &metricsResponse); err != nil {
		return nil, err
	}
	return &metricsResponse, nil
}

func (c *actuatorClient) GetMetric(metricName string) (*MetricResponse, error) {
	path := "/metrics/" + url.PathEscape(metricName)
	resp, err := c.httpClient.Get(path)
	if err != nil {
		return nil, err
	}

	if resp.IsErrorStatus() {
		if resp.StatusCode == 404 && c.isEndpointAccessible("/metrics") {
			return nil, resourceNotFoundError("metric", metricName, resp.Status)
		}
		return nil, endpointError("metrics", resp.Status, "failed to get metric")
	}

	var metricResponse MetricResponse
	if err := parseJSON(resp.Body, &metricResponse); err != nil {
		return nil, err
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
