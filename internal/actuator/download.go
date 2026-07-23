package actuator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// DownloadHeapDump streams a JVM heap dump from the /heapdump endpoint into
// writer and returns the number of bytes written. When live is nil, no live
// parameter is sent and the JVM default applies (OpenJ9 rejects an explicit
// value, so nil must stay nil).
func (c *actuatorClient) DownloadHeapDump(writer io.Writer, live *bool) (int64, error) {
	path := "/heapdump"
	if live != nil {
		path += "?live=" + strconv.FormatBool(*live)
	}

	resp, err := c.httpClient.Stream(path, nil)
	if err != nil {
		return 0, c.connectionError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsErrorStatus() {
		switch resp.StatusCode {
		case 429:
			return 0, fmt.Errorf("another heap dump is already in progress: %s", resp.Status)
		case 503:
			return 0, fmt.Errorf("heap dump is not available on this JVM: %s", resp.Status)
		default:
			return 0, c.statusError("heapdump", resp.StatusCode, resp.Status, "failed to get heap dump")
		}
	}

	return copyBody(writer, resp.Body)
}

// DownloadLogFile streams the application log file from the /logfile endpoint
// into writer and returns the number of bytes written. When tailBytes is
// positive, only the last tailBytes bytes are requested via an HTTP Range
// header.
func (c *actuatorClient) DownloadLogFile(writer io.Writer, tailBytes int64) (int64, error) {
	var headers map[string]string
	if tailBytes > 0 {
		headers = map[string]string{"Range": fmt.Sprintf("bytes=-%d", tailBytes)}
	}

	resp, err := c.httpClient.Stream("/logfile", headers)
	if err != nil {
		return 0, c.connectionError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsErrorStatus() {
		return 0, c.statusError("logfile", resp.StatusCode, resp.Status, "failed to get log file")
	}

	return copyBody(writer, resp.Body)
}

// copyBody streams a response body into writer, keeping an interrupt
// recognizable instead of burying it in a transport error.
func copyBody(writer io.Writer, body io.Reader) (int64, error) {
	written, err := io.Copy(writer, body)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return written, context.Canceled
		}
		return written, fmt.Errorf("download failed after %d bytes: %w", written, err)
	}
	return written, nil
}
