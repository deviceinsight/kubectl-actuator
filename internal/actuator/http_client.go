package actuator

import (
	"context"
	"encoding/json"
	"io"

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

// StreamResponse is a response whose body is streamed rather than buffered.
// The caller must close Body.
type StreamResponse struct {
	Body       io.ReadCloser
	StatusCode int
	Status     string
}

func (r *StreamResponse) IsErrorStatus() bool {
	return r.StatusCode < 200 || r.StatusCode >= 300
}

type restyHTTPClient struct {
	// ctx is the invoking command's context, attached to every request so an
	// interrupt aborts in-flight calls. It is stored rather than passed per
	// call because the Client interface predates per-call contexts and a
	// client only lives for one command invocation.
	ctx   context.Context
	resty *resty.Client
	// stream has no overall timeout: http.Client.Timeout covers reading the
	// entire body, which would abort large downloads like heap dumps.
	stream *resty.Client
}

var _ HTTPClient = (*restyHTTPClient)(nil)

func newRestyHTTPClient(ctx context.Context, client, streamClient *resty.Client) HTTPClient {
	return &restyHTTPClient{ctx: ctx, resty: client, stream: streamClient}
}

func (c *restyHTTPClient) Get(path string) (*Response, error) {
	response, err := c.resty.R().SetContext(c.ctx).Get(path)
	if err != nil {
		return nil, err
	}
	return &Response{
		Body:       response.Body(),
		StatusCode: response.StatusCode(),
		Status:     response.Status(),
	}, nil
}

func (c *restyHTTPClient) Stream(path string, headers map[string]string) (*StreamResponse, error) {
	request := c.stream.R().SetContext(c.ctx).SetDoNotParseResponse(true)
	for key, value := range headers {
		request.SetHeader(key, value)
	}
	response, err := request.Get(path)
	if err != nil {
		return nil, err
	}
	return &StreamResponse{
		Body:       response.RawBody(),
		StatusCode: response.StatusCode(),
		Status:     response.Status(),
	}, nil
}

func (c *restyHTTPClient) Post(path string, body any) (*Response, error) {
	response, err := c.resty.R().SetContext(c.ctx).SetBody(body).Post(path)
	if err != nil {
		return nil, err
	}
	return &Response{
		Body:       response.Body(),
		StatusCode: response.StatusCode(),
		Status:     response.Status(),
	}, nil
}

func parseJSON(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
