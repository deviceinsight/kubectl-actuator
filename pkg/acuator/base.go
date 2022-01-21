package acuator

import (
	"github.com/go-resty/resty/v2"
	"net/http"
)

type ActuatorClient struct {
	resty *resty.Client
}

func BuildClient(transport *http.Transport, baseUrl string) *ActuatorClient {
	client := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL("http://port-forwarded-actuator/" + baseUrl)

	return &ActuatorClient{resty: client}
}
