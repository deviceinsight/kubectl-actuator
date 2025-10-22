package actuator

import "context"

type Client interface {
	GetLoggers() ([]LoggerConfiguration, error)
	SetLoggerLevel(logger string, level string) error
	GetScheduledTasks() (*ScheduledTasksResponse, error)
	GetInfo() (map[string]interface{}, error)
}

type HTTPClient interface {
	Get(path string) (*Response, error)
	Post(path string, body interface{}) (*Response, error)
}

type ClientFactory interface {
	NewClient(ctx context.Context, podName string) (Client, error)
}
