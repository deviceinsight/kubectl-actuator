package actuator

import (
	"encoding/json"

	"github.com/go-resty/resty/v2"
)

type Response struct {
	Body       []byte
	StatusCode int
	Status     string
}

func (r *Response) IsErrorStatus() bool {
	return r.StatusCode < 200 || r.StatusCode >= 300
}

type restyHTTPClient struct {
	resty *resty.Client
}

func newRestyHTTPClient(client *resty.Client) HTTPClient {
	return &restyHTTPClient{resty: client}
}

func (c *restyHTTPClient) Get(path string) (*Response, error) {
	response, err := c.resty.R().Get(path)
	if err != nil {
		return nil, err
	}
	return &Response{
		Body:       response.Body(),
		StatusCode: response.StatusCode(),
		Status:     response.Status(),
	}, nil
}

func (c *restyHTTPClient) Post(path string, body interface{}) (*Response, error) {
	response, err := c.resty.R().SetBody(body).Post(path)
	if err != nil {
		return nil, err
	}
	return &Response{
		Body:       response.Body(),
		StatusCode: response.StatusCode(),
		Status:     response.Status(),
	}, nil
}

func parseJSON(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
