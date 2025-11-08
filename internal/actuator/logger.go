package actuator

import "net/url"

func (c *actuatorClient) GetLoggers() ([]LoggerConfiguration, error) {
	var actuatorResponse loggersResponse
	if err := c.getAndParse("/loggers", "loggers", "failed to get loggers", &actuatorResponse); err != nil {
		return nil, err
	}

	var loggers []LoggerConfiguration
	for loggerName, logger := range actuatorResponse.Loggers {
		loggers = append(loggers, LoggerConfiguration{
			Name:            loggerName,
			ConfiguredLevel: logger.ConfiguredLevel,
			EffectiveLevel:  logger.EffectiveLevel,
		})
	}

	return loggers, nil
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
		return err
	}

	if resp.IsErrorStatus() {
		return endpointError("loggers", resp.Status, "failed to set logger level")
	}

	return nil
}

type LoggerConfiguration struct {
	Name            string
	ConfiguredLevel *string
	EffectiveLevel  *string
}

type setLoggerLevelRequest struct {
	ConfiguredLevel *string `json:"configuredLevel"`
}

type loggersResponse struct {
	Loggers map[string]loggerInfo `json:"loggers"`
}

type loggerInfo struct {
	ConfiguredLevel *string `json:"configuredLevel"`
	EffectiveLevel  *string `json:"effectiveLevel"`
}
