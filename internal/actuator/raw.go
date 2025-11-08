package actuator

func (c *actuatorClient) GetRaw(endpoint string) ([]byte, error) {
	// Ensure endpoint starts with / (unless it's empty for the root endpoint)
	if endpoint != "" && endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}

	if resp.IsErrorStatus() {
		return nil, endpointError(endpoint, resp.Status, "failed to get endpoint")
	}

	return resp.Body, nil
}
