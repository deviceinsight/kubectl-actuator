package actuator

import (
	"net/url"
	"sort"
)

func (c *actuatorClient) GetLoggers() ([]LoggerConfiguration, error) {
	resp, err := c.httpClient.Get("/loggers")
	if err != nil {
		return nil, err
	}
	if resp.IsErrorStatus() {
		return nil, endpointError("loggers", resp.Status, "unable to get loggers")
	}

	var actuatorResponse loggersResponse
	if err := parseJSON(resp.Body, &actuatorResponse); err != nil {
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

	sort.Slice(loggers, func(i, j int) bool {
		return loggers[i].Name < loggers[j].Name
	})

	return loggers, nil
}

func (c *actuatorClient) SetLoggerLevel(logger string, level string) error {
	path := "/loggers/" + url.PathEscape(logger)
	body := setLoggerLevelRequest{
		ConfiguredLevel: &level,
	}

	resp, err := c.httpClient.Post(path, body)
	if err != nil {
		return err
	}

	if resp.IsErrorStatus() {
		return endpointError("loggers", resp.Status, "unable to set logger level")
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
