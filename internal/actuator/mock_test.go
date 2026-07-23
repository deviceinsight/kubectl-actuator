package actuator

import (
	"io"
	"strconv"
	"strings"
	"testing"
)

// MockHTTPClient is the shared fake for the actuator client's HTTP layer.
type MockHTTPClient struct {
	GetFunc    func(path string) (*Response, error)
	PostFunc   func(path string, body any) (*Response, error)
	StreamFunc func(path string, headers map[string]string) (*StreamResponse, error)
}

func (m *MockHTTPClient) Get(path string) (*Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(path)
	}
	return &Response{Body: nil, StatusCode: 200, Status: "200 OK"}, nil
}

func (m *MockHTTPClient) Post(path string, body any) (*Response, error) {
	if m.PostFunc != nil {
		return m.PostFunc(path, body)
	}
	return &Response{Body: nil, StatusCode: 200, Status: "200 OK"}, nil
}

func (m *MockHTTPClient) Stream(path string, headers map[string]string) (*StreamResponse, error) {
	if m.StreamFunc != nil {
		return m.StreamFunc(path, headers)
	}
	return &StreamResponse{Body: io.NopCloser(strings.NewReader("")), StatusCode: 200, Status: "200 OK"}, nil
}

// newEndpointMock serves a single endpoint path and answers the actuator
// index probes of the 404 diagnosis with 404; any other path fails the test.
func newEndpointMock(t *testing.T, endpointPath string, respond func() (*Response, error)) *MockHTTPClient {
	t.Helper()
	return &MockHTTPClient{
		GetFunc: func(path string) (*Response, error) {
			if path == "" || strings.HasPrefix(path, portForwardBaseURL) {
				return &Response{StatusCode: 404, Status: "404"}, nil
			}
			if path != endpointPath {
				t.Errorf("unexpected path: %s", path)
			}
			return respond()
		},
	}
}

func respondWith(status int, body string) func() (*Response, error) {
	return func() (*Response, error) {
		return &Response{Body: []byte(body), StatusCode: status, Status: strconv.Itoa(status)}, nil
	}
}
