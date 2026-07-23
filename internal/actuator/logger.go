package actuator

import "net/url"

func (c *actuatorClient) GetLoggers() (*LoggersResponse, error) {
	var payload loggersPayload
	if err := c.getAndParse("/loggers", "loggers", "failed to get loggers", &payload); err != nil {
		return nil, err
	}

	response := &LoggersResponse{}
	for loggerName, logger := range payload.Loggers {
		response.Loggers = append(response.Loggers, LoggerConfiguration{
			Name:            loggerName,
			ConfiguredLevel: logger.ConfiguredLevel,
			EffectiveLevel:  logger.EffectiveLevel,
		})
	}
	for groupName, group := range payload.Groups {
		response.Groups = append(response.Groups, LoggerGroup{
			Name:            groupName,
			ConfiguredLevel: group.ConfiguredLevel,
			Members:         group.Members,
		})
	}

	return response, nil
}

func (c *actuatorClient) SetLoggerLevel(logger string, level string) error {
	path := "/loggers/" + url.PathEscape(logger)
	var body setLoggerLevelRequest
	if level == "" {
		// Send null to reset logger to inherited level
		body = setLoggerLevelRequest{ConfiguredLevel: nil}
	} else {
		body = setLoggerLevelRequest{ConfiguredLevel: &level}
	}

	resp, err := c.httpClient.Post(path, body)
	if err != nil {
		return c.connectionError(err)
	}

	if resp.IsErrorStatus() {
		return c.statusError("loggers", resp.StatusCode, resp.Status, "failed to set logger level")
	}

	return nil
}

// LoggersResponse is the parsed /loggers document: individual loggers plus
// logger groups (Spring Boot 2.7+), which set the level of all their
// members at once.
type LoggersResponse struct {
	Loggers []LoggerConfiguration
	Groups  []LoggerGroup
}

type LoggerConfiguration struct {
	Name            string
	ConfiguredLevel *string
	EffectiveLevel  *string
}

type LoggerGroup struct {
	Name            string
	ConfiguredLevel *string
	Members         []string
}

type setLoggerLevelRequest struct {
	ConfiguredLevel *string `json:"configuredLevel"`
}

type loggersPayload struct {
	Loggers map[string]loggerInfo `json:"loggers"`
	Groups  map[string]groupInfo  `json:"groups"`
}

type loggerInfo struct {
	ConfiguredLevel *string `json:"configuredLevel"`
	EffectiveLevel  *string `json:"effectiveLevel"`
}

type groupInfo struct {
	ConfiguredLevel *string  `json:"configuredLevel"`
	Members         []string `json:"members"`
}
