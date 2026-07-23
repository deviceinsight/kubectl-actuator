package actuator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const endpointsDocsURL = "https://docs.spring.io/spring-boot/reference/actuator/endpoints.html"

// endpointActivationHints holds extra per-endpoint guidance shown when the
// endpoint is missing from the actuator index.
var endpointActivationHints = map[string]string{
	"heapdump": "Since Spring Boot 3.5 it also requires management.endpoint.heapdump.access=unrestricted.",
	"logfile":  "The logfile endpoint only exists when logging.file.name or logging.file.path is set in the application.",
}

// notFoundDiagnosis is the result of probing the actuator index after a 404.
type notFoundDiagnosis int

const (
	// actuatorUnreachable: the index itself does not respond; the port or
	// base path is wrong, or the actuator is disabled entirely.
	actuatorUnreachable notFoundDiagnosis = iota
	// endpointNotExposed: the index responds but does not list the endpoint.
	endpointNotExposed
	// endpointExposedAnyway: the index lists the endpoint, so the 404 is
	// about the requested resource, not the endpoint.
	endpointExposedAnyway
)

// classifyNotFound probes the actuator index to determine why an endpoint
// request returned 404. It costs one extra request, on error paths only.
func (c *actuatorClient) classifyNotFound(endpointID string) notFoundDiagnosis {
	index, err := c.getActuatorIndex()
	if err != nil {
		return actuatorUnreachable
	}
	if _, ok := index.Links[endpointID]; ok {
		return endpointExposedAnyway
	}
	return endpointNotExposed
}

// statusError builds a user-facing error for a non-2xx response, making sure
// guidance matches the actual cause: connectivity and configuration problems
// are never reported as disabled endpoints.
func (c *actuatorClient) statusError(endpointID string, statusCode int, status, messagePrefix string) error {
	switch {
	case statusCode == 404:
		return c.notFoundError(endpointID, status, messagePrefix)
	case statusCode == 401 || statusCode == 403:
		return fmt.Errorf("%s: the actuator responded with HTTP %s\nActuator endpoints appear to be secured; kubectl-actuator sends unauthenticated requests. Check the application's security configuration for actuator endpoints.", messagePrefix, status)
	case statusCode >= 500:
		return fmt.Errorf("%s: the %q endpoint returned a server error: %s", messagePrefix, endpointID, status)
	default:
		return fmt.Errorf("%s: unexpected response from the %q endpoint: %s", messagePrefix, endpointID, status)
	}
}

// resourceStatusError builds the error for a non-2xx response to a request
// for one resource of an endpoint (a property, a metric, a health group).
// A 404 is classified via the actuator index: when the endpoint itself is
// exposed, notFoundErr describes the missing resource; otherwise the regular
// endpoint diagnosis applies.
func (c *actuatorClient) resourceStatusError(endpointID string, statusCode int, status, messagePrefix string, notFoundErr error) error {
	if statusCode != 404 {
		return c.statusError(endpointID, statusCode, status, messagePrefix)
	}
	switch c.classifyNotFound(endpointID) {
	case endpointNotExposed:
		return c.notExposedError(endpointID, messagePrefix)
	case endpointExposedAnyway:
		return notFoundErr
	case actuatorUnreachable:
		return c.actuatorUnreachableError(status, messagePrefix)
	}
	panic("unreachable")
}

func resourceNotFoundError(resourceType string, resourceName string, status string) error {
	return fmt.Errorf("%s %q not found: %s", resourceType, resourceName, status)
}

func (c *actuatorClient) notFoundError(endpointID, status, messagePrefix string) error {
	switch c.classifyNotFound(endpointID) {
	case endpointNotExposed:
		return c.notExposedError(endpointID, messagePrefix)
	case endpointExposedAnyway:
		return fmt.Errorf("%s: HTTP %s although the %q endpoint is exposed", messagePrefix, status, endpointID)
	case actuatorUnreachable:
		return c.actuatorUnreachableError(status, messagePrefix)
	}
	panic("unreachable")
}

func (c *actuatorClient) notExposedError(endpointID, messagePrefix string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: the %q endpoint is not exposed by this application\n", messagePrefix, endpointID)
	b.WriteString("Expose it via management.endpoints.web.exposure.include (Spring Boot exposes only 'health' by default).")
	if hint, ok := endpointActivationHints[endpointID]; ok {
		b.WriteString("\n" + hint)
	}
	b.WriteString("\nDocs: " + endpointsDocsURL)
	return fmt.Errorf("%s", b.String())
}

func (c *actuatorClient) actuatorUnreachableError(status, messagePrefix string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: no Spring Boot Actuator found at port %d (%s), base path %q (%s): HTTP %s\n",
		messagePrefix, c.port, c.portSource, c.basePathDisplay(), c.basePathSource, status)
	fmt.Fprintf(&b, "Check --port and --base-path or the pod annotations (%s, %s).\n", portAnnotation, basePathAnnotation)
	b.WriteString("If they are correct, actuator web exposure may be disabled entirely.")
	if c.basePath != defaultBasePath && c.indexRespondsAtDefaultBasePath() {
		fmt.Fprintf(&b, "\nHint: an actuator index responds at the default base path %q.", defaultBasePath)
	}
	return fmt.Errorf("%s", b.String())
}

// indexRespondsAtDefaultBasePath probes the default base path with an
// absolute URL, bypassing the configured base path.
func (c *actuatorClient) indexRespondsAtDefaultBasePath() bool {
	resp, err := c.httpClient.Get(portForwardBaseURL + "/" + defaultBasePath)
	if err != nil || resp.IsErrorStatus() {
		return false
	}
	var index actuatorIndex
	if err := parseJSON(resp.Body, &index); err != nil {
		return false
	}
	return len(index.Links) > 0
}

// notJSONError explains a 2xx response whose body is not the JSON an
// actuator produces: most likely something other than Spring Boot Actuator
// (often the application itself) answered at the resolved port and base path.
func (c *actuatorClient) notJSONError(parseErr error, messagePrefix string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: the response is not valid JSON: %v\n", messagePrefix, parseErr)
	fmt.Fprintf(&b, "The server at port %d (%s), base path %q (%s) does not appear to be a Spring Boot Actuator.\n",
		c.port, c.portSource, c.basePathDisplay(), c.basePathSource)
	fmt.Fprintf(&b, "Check --port and --base-path or the pod annotations (%s, %s).", portAnnotation, basePathAnnotation)
	return fmt.Errorf("%s", b.String())
}

// basePathDisplay renders the base path for error messages; the root base
// path (empty after normalization) is shown as '/'.
func (c *actuatorClient) basePathDisplay() string {
	if c.basePath == "" {
		return "/"
	}
	return c.basePath
}

// connectionError wraps transport-level failures with the resolved port so a
// misconfigured port is diagnosable. A canceled context is the user's
// interrupt, not a connectivity problem, and gets no port guidance; a
// timeout names the client-side limit that fired instead of blaming the
// port.
func (c *actuatorClient) connectionError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}

	// Strip *url.Error wrappers: they embed the placeholder port-forward
	// hostname, which reads like a DNS problem.
	var urlErr *url.Error
	for errors.As(err, &urlErr) {
		if urlErr.Err == nil {
			break
		}
		err = urlErr.Err
	}

	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("no response from the actuator on port %d (%s) within %s; if the endpoint is just slow, raise --timeout", c.port, c.portSource, c.timeout)
	}

	return fmt.Errorf("failed to reach the actuator on port %d (%s): %w\nIf the port is wrong, set --port or the %s annotation.", c.port, c.portSource, err, portAnnotation)
}
