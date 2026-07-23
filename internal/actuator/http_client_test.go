package actuator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestResponseIsErrorStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		wantError  bool
	}{
		{200, false},
		{201, false},
		{204, false},
		{400, true},
		{401, true},
		{404, true},
		{500, true},
		{502, true},
	}

	for _, tt := range tests {
		t.Run("status_"+strconv.Itoa(tt.statusCode), func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			if got := resp.IsErrorStatus(); got != tt.wantError {
				t.Errorf("Response.IsErrorStatus() with status %d = %v, want %v", tt.statusCode, got, tt.wantError)
			}
		})
	}
}

// TestHTTPClientAppliesContext verifies every request carries the command's
// context, so an interrupt aborts in-flight calls instead of being swallowed.
func TestHTTPClientAppliesContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newRestyHTTPClient(ctx, resty.New().SetBaseURL(server.URL), resty.New().SetBaseURL(server.URL))

	if _, err := client.Get("/"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get: expected context.Canceled, got %v", err)
	}
	if _, err := client.Post("/", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Post: expected context.Canceled, got %v", err)
	}
	if _, err := client.Stream("/", nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Stream: expected context.Canceled, got %v", err)
	}
}
