package actuator

func (c *actuatorClient) GetScheduledTasks() (*ScheduledTasksResponse, error) {
	var tasksResponse ScheduledTasksResponse
	if err := c.getAndParse("/scheduledtasks", "scheduledtasks", "failed to get scheduled tasks", &tasksResponse); err != nil {
		return nil, err
	}
	return &tasksResponse, nil
}

type ScheduledTasksResponse struct {
	Cron       []CronTask          `json:"cron"`
	FixedDelay []FixedIntervalTask `json:"fixedDelay"`
	FixedRate  []FixedIntervalTask `json:"fixedRate"`
	Custom     []CustomTask        `json:"custom"`
}

type Runnable struct {
	Target string `json:"target"`
}

// TimeOnly is an execution that has not happened yet: Spring reports only its
// scheduled time.
type TimeOnly struct {
	Time string `json:"time"`
}

type Exception struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type Execution struct {
	Time      string     `json:"time"`
	Status    string     `json:"status,omitempty"`
	Exception *Exception `json:"exception,omitempty"`
}

type CronTask struct {
	Runnable      Runnable   `json:"runnable"`
	Expression    string     `json:"expression"`
	NextExecution *TimeOnly  `json:"nextExecution,omitempty"`
	LastExecution *Execution `json:"lastExecution,omitempty"`
}

type FixedIntervalTask struct {
	Runnable      Runnable   `json:"runnable"`
	InitialDelay  int64      `json:"initialDelay"`
	Interval      int64      `json:"interval"`
	NextExecution *TimeOnly  `json:"nextExecution,omitempty"`
	LastExecution *Execution `json:"lastExecution,omitempty"`
}

type CustomTask struct {
	Runnable      Runnable   `json:"runnable"`
	NextExecution *TimeOnly  `json:"nextExecution,omitempty"`
	LastExecution *Execution `json:"lastExecution,omitempty"`
}
