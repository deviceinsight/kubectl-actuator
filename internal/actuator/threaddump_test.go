package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetThreadDump(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   string
		mockStatus     int
		mockErr        error
		wantErr        bool
		wantThreadsCnt int
	}{
		{
			name: "successful response with threads",
			mockResponse: `{
				"threads": [
					{
						"threadName": "main",
						"threadId": 1,
						"threadState": "RUNNABLE",
						"blockedCount": 0,
						"blockedTime": -1,
						"waitedCount": 0,
						"waitedTime": -1,
						"lockOwnerId": -1,
						"daemon": false,
						"inNative": false,
						"suspended": false,
						"priority": 5,
						"stackTrace": [
							{
								"className": "java.lang.Thread",
								"methodName": "sleep",
								"fileName": "Thread.java",
								"lineNumber": 340,
								"nativeMethod": true
							}
						],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					},
					{
						"threadName": "http-nio-8080-exec-1",
						"threadId": 42,
						"threadState": "WAITING",
						"blockedCount": 5,
						"blockedTime": -1,
						"waitedCount": 100,
						"waitedTime": -1,
						"lockOwnerId": -1,
						"daemon": true,
						"inNative": false,
						"suspended": false,
						"priority": 5,
						"stackTrace": [],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					}
				]
			}`,
			mockStatus:     200,
			wantErr:        false,
			wantThreadsCnt: 2,
		},
		{
			name:           "empty threads list",
			mockResponse:   `{"threads": []}`,
			mockStatus:     200,
			wantErr:        false,
			wantThreadsCnt: 0,
		},
		{
			name:         "404 endpoint not found",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
		},
		{
			name:         "500 internal server error",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
		},
		{
			name:         "malformed JSON",
			mockResponse: `{"threads": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path != "/threaddump" {
						t.Errorf("unexpected path: %s", path)
					}
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return &Response{
						Body:       []byte(tt.mockResponse),
						StatusCode: tt.mockStatus,
						Status:     strconv.Itoa(tt.mockStatus),
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetThreadDump()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetThreadDump() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Threads) != tt.wantThreadsCnt {
					t.Errorf("got %d threads, want %d", len(result.Threads), tt.wantThreadsCnt)
				}
			}
		})
	}
}

func TestThreadDumpResponseParsing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		validate func(*testing.T, *ThreadDumpResponse)
	}{
		{
			name: "thread with all fields",
			response: `{
				"threads": [
					{
						"threadName": "worker-thread-1",
						"threadId": 123,
						"threadState": "BLOCKED",
						"blockedCount": 10,
						"blockedTime": 500,
						"waitedCount": 20,
						"waitedTime": 1000,
						"lockOwnerId": 456,
						"daemon": true,
						"inNative": true,
						"suspended": true,
						"priority": 10,
						"stackTrace": [
							{
								"className": "com.example.Worker",
								"methodName": "process",
								"fileName": "Worker.java",
								"lineNumber": 42,
								"nativeMethod": false
							},
							{
								"className": "com.example.Main",
								"methodName": "run",
								"fileName": "Main.java",
								"lineNumber": 10,
								"nativeMethod": false
							}
						],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					}
				]
			}`,
			validate: func(t *testing.T, resp *ThreadDumpResponse) {
				if len(resp.Threads) != 1 {
					t.Fatalf("expected 1 thread, got %d", len(resp.Threads))
				}
				thread := resp.Threads[0]
				if thread.ThreadName != "worker-thread-1" {
					t.Errorf("expected threadName 'worker-thread-1', got '%s'", thread.ThreadName)
				}
				if thread.ThreadID != 123 {
					t.Errorf("expected threadId 123, got %d", thread.ThreadID)
				}
				if thread.ThreadState != "BLOCKED" {
					t.Errorf("expected threadState 'BLOCKED', got '%s'", thread.ThreadState)
				}
				if !thread.Daemon {
					t.Error("expected daemon to be true")
				}
				if !thread.InNative {
					t.Error("expected inNative to be true")
				}
				if len(thread.StackTrace) != 2 {
					t.Errorf("expected 2 stack frames, got %d", len(thread.StackTrace))
				}
			},
		},
		{
			name: "thread states",
			response: `{
				"threads": [
					{"threadName": "t1", "threadId": 1, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t2", "threadId": 2, "threadState": "WAITING", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t3", "threadId": 3, "threadState": "TIMED_WAITING", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t4", "threadId": 4, "threadState": "BLOCKED", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []}
				]
			}`,
			validate: func(t *testing.T, resp *ThreadDumpResponse) {
				if len(resp.Threads) != 4 {
					t.Fatalf("expected 4 threads, got %d", len(resp.Threads))
				}
				states := []string{"RUNNABLE", "WAITING", "TIMED_WAITING", "BLOCKED"}
				for i, state := range states {
					if resp.Threads[i].ThreadState != state {
						t.Errorf("thread[%d] expected state '%s', got '%s'", i, state, resp.Threads[i].ThreadState)
					}
				}
			},
		},
		{
			name: "stack frame with null fileName and lineNumber",
			response: `{
				"threads": [
					{
						"threadName": "native-thread",
						"threadId": 1,
						"threadState": "RUNNABLE",
						"blockedCount": 0,
						"blockedTime": 0,
						"waitedCount": 0,
						"waitedTime": 0,
						"lockOwnerId": -1,
						"daemon": false,
						"inNative": true,
						"suspended": false,
						"priority": 5,
						"stackTrace": [
							{
								"className": "sun.misc.Unsafe",
								"methodName": "park",
								"fileName": null,
								"lineNumber": null,
								"nativeMethod": true
							}
						],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					}
				]
			}`,
			validate: func(t *testing.T, resp *ThreadDumpResponse) {
				thread := resp.Threads[0]
				if len(thread.StackTrace) != 1 {
					t.Fatalf("expected 1 stack frame, got %d", len(thread.StackTrace))
				}
				frame := thread.StackTrace[0]
				if frame.FileName != nil {
					t.Errorf("expected nil fileName, got %v", *frame.FileName)
				}
				if frame.LineNumber != nil {
					t.Errorf("expected nil lineNumber, got %v", *frame.LineNumber)
				}
				if !frame.NativeMethod {
					t.Error("expected nativeMethod to be true")
				}
			},
		},
		{
			name: "many threads",
			response: `{
				"threads": [
					{"threadName": "t1", "threadId": 1, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t2", "threadId": 2, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t3", "threadId": 3, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t4", "threadId": 4, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []},
					{"threadName": "t5", "threadId": 5, "threadState": "RUNNABLE", "blockedCount": 0, "blockedTime": 0, "waitedCount": 0, "waitedTime": 0, "lockOwnerId": -1, "daemon": false, "inNative": false, "suspended": false, "priority": 5, "stackTrace": [], "lockedMonitors": [], "lockedSynchronizers": []}
				]
			}`,
			validate: func(t *testing.T, resp *ThreadDumpResponse) {
				if len(resp.Threads) != 5 {
					t.Errorf("expected 5 threads, got %d", len(resp.Threads))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					return &Response{
						Body:       []byte(tt.response),
						StatusCode: 200,
						Status:     "200",
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetThreadDump()

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validate(t, result)
		})
	}
}
