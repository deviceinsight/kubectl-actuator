package acuator

import "errors"

func (c ActuatorClient) GetLoggers() ([]LoggerConfiguration, error) {
	response, err := c.resty.R().
		SetResult(loggersResponse{}).
		Get("/loggers/")
	if err != nil {
		return nil, err
	}

	actuatorResponse := response.Result().(*loggersResponse)

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

func (c ActuatorClient) SetLoggerLevel(logger string, level *string) error {
	response, err := c.resty.R().
		SetPathParams(map[string]string{
			"logger": logger,
		}).
		SetBody(setLoggerLevelRequest{
			ConfiguredLevel: level,
		}).
		Post("/loggers/{logger}")
	if err != nil {
		return err
	}

	if response.IsError() {
		return errors.New("Unexpected HTTP response status: " + response.Status())
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
