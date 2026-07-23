package actuator

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func streamResponse(body string, status int) *StreamResponse {
	return &StreamResponse{
		Body:       io.NopCloser(strings.NewReader(body)),
		StatusCode: status,
		Status:     "status",
	}
}

func TestActuatorClientDownloadHeapDump(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name        string
		live        *bool
		status      int
		wantPath    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "default live parameter omitted",
			live:     nil,
			status:   200,
			wantPath: "/heapdump",
		},
		{
			name:     "live true",
			live:     boolPtr(true),
			status:   200,
			wantPath: "/heapdump?live=true",
		},
		{
			name:     "live false",
			live:     boolPtr(false),
			status:   200,
			wantPath: "/heapdump?live=false",
		},
		{
			name:        "429 concurrent dump",
			status:      429,
			wantPath:    "/heapdump",
			wantErr:     true,
			errContains: "already in progress",
		},
		{
			name:        "503 dumper unavailable",
			status:      503,
			wantPath:    "/heapdump",
			wantErr:     true,
			errContains: "not available on this JVM",
		},
		{
			name:        "404 endpoint not accessible",
			status:      404,
			wantPath:    "/heapdump",
			wantErr:     true,
			errContains: "management.endpoint.heapdump.access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPath string
			mockClient := &MockHTTPClient{
				StreamFunc: func(path string, headers map[string]string) (*StreamResponse, error) {
					requestedPath = path
					return streamResponse("HPROF-DATA", tt.status), nil
				},
				// 404 diagnosis probes the index; it lists no heapdump link
				GetFunc: func(path string) (*Response, error) {
					return &Response{Body: []byte(`{"_links": {"self": {"href": "/actuator"}}}`), StatusCode: 200, Status: "200"}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			var buf bytes.Buffer
			written, err := client.DownloadHeapDump(&buf, tt.live)

			if requestedPath != tt.wantPath {
				t.Errorf("requested path = %q, want %q", requestedPath, tt.wantPath)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("DownloadHeapDump() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if written != int64(len("HPROF-DATA")) || buf.String() != "HPROF-DATA" {
				t.Errorf("written = %d, body = %q", written, buf.String())
			}
		})
	}
}

func TestActuatorClientDownloadLogFile(t *testing.T) {
	tests := []struct {
		name        string
		tailBytes   int64
		status      int
		wantRange   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "whole file without range header",
			tailBytes: 0,
			status:    200,
		},
		{
			name:      "tail sends range header",
			tailBytes: 4096,
			status:    206,
			wantRange: "bytes=-4096",
		},
		{
			name:        "404 no log file",
			status:      404,
			wantErr:     true,
			errContains: "logging.file.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPath, rangeHeader string
			mockClient := &MockHTTPClient{
				StreamFunc: func(path string, headers map[string]string) (*StreamResponse, error) {
					requestedPath = path
					rangeHeader = headers["Range"]
					return streamResponse("2026-07-23 INFO Started", tt.status), nil
				},
				// 404 diagnosis probes the index; it lists no logfile link
				GetFunc: func(path string) (*Response, error) {
					return &Response{Body: []byte(`{"_links": {"self": {"href": "/actuator"}}}`), StatusCode: 200, Status: "200"}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			var buf bytes.Buffer
			written, err := client.DownloadLogFile(&buf, tt.tailBytes)

			if requestedPath != "/logfile" {
				t.Errorf("requested path = %q, want /logfile", requestedPath)
			}
			if rangeHeader != tt.wantRange {
				t.Errorf("Range header = %q, want %q", rangeHeader, tt.wantRange)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("DownloadLogFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if written == 0 || buf.Len() == 0 {
				t.Error("expected body to be written")
			}
		})
	}
}
