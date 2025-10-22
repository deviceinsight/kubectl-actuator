package actuator

func (c *actuatorClient) GetInfo() (map[string]interface{}, error) {
	resp, err := c.httpClient.Get("/info")
	if err != nil {
		return nil, err
	}

	if resp.IsErrorStatus() {
		return nil, endpointError("info", resp.Status, "failed to get info")
	}

	var infoResponse map[string]interface{}
	if err := parseJSON(resp.Body, &infoResponse); err != nil {
		return nil, err
	}

	return infoResponse, nil
}
