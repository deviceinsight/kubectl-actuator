package actuator

func (c *actuatorClient) GetInfo() (map[string]interface{}, error) {
	var infoResponse map[string]interface{}
	if err := c.getAndParse("/info", "info", "failed to get info", &infoResponse); err != nil {
		return nil, err
	}
	return infoResponse, nil
}
