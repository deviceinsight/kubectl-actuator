package actuator

func (c *actuatorClient) GetThreadDump() (*ThreadDumpResponse, error) {
	var threadDumpResponse ThreadDumpResponse
	if err := c.getAndParse("/threaddump", "threaddump", "failed to get thread dump", &threadDumpResponse); err != nil {
		return nil, err
	}
	return &threadDumpResponse, nil
}

type ThreadDumpResponse struct {
	Threads []Thread `json:"threads"`
}

type Thread struct {
	ThreadName          string        `json:"threadName"`
	ThreadID            int64         `json:"threadId"`
	ThreadState         string        `json:"threadState"`
	BlockedCount        int64         `json:"blockedCount"`
	BlockedTime         int64         `json:"blockedTime"`
	WaitedCount         int64         `json:"waitedCount"`
	WaitedTime          int64         `json:"waitedTime"`
	LockOwnerId         int64         `json:"lockOwnerId"`
	Daemon              bool          `json:"daemon"`
	InNative            bool          `json:"inNative"`
	Suspended           bool          `json:"suspended"`
	Priority            int           `json:"priority"`
	StackTrace          []StackFrame  `json:"stackTrace"`
	LockedMonitors      []interface{} `json:"lockedMonitors"`
	LockedSynchronizers []interface{} `json:"lockedSynchronizers"`
}

type StackFrame struct {
	ClassName    string  `json:"className"`
	MethodName   string  `json:"methodName"`
	FileName     *string `json:"fileName"`
	LineNumber   *int    `json:"lineNumber"`
	NativeMethod bool    `json:"nativeMethod"`
}
