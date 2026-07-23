package actuator

import "strings"

func (c *actuatorClient) GetRaw(endpoint string) ([]byte, error) {
	if endpoint != "" && endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, c.connectionError(err)
	}

	if resp.IsErrorStatus() {
		if id := rawEndpointID(endpoint); id != "" {
			return nil, c.statusError(id, resp.StatusCode, resp.Status, "failed to get endpoint")
		}
		return nil, c.actuatorUnreachableError(resp.Status, "failed to get actuator index")
	}

	return resp.Body, nil
}

// rawEndpointID extracts the endpoint id from a raw request path such as
// "/loggers/com.example" or "metrics/jvm.memory.used?tag=area:heap".
func rawEndpointID(endpoint string) string {
	id := strings.TrimPrefix(endpoint, "/")
	if idx := strings.IndexAny(id, "/?"); idx != -1 {
		id = id[:idx]
	}
	return id
}
