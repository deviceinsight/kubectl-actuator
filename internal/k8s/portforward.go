package k8s

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/transport/spdy"
)

var nextPortForwardRequestID uint64

func (c *Connection) CreateHttpTransport(podName string, podPort int) (*http.Transport, error) {
	portForwardURL := c.restClient.Post().
		Resource("pods").
		Namespace(c.namespace).
		Name(podName).
		SubResource("portforward").
		URL()
	baseTransport, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return nil, err
	}

	return &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: baseTransport}, "POST", portForwardURL)
			conn, _, err := dialer.Dial("portforward.k8s.io")
			if err != nil {
				return nil, fmt.Errorf("unable to dial portforward protocol: %w", err)
			}

			id := strconv.FormatUint(atomic.AddUint64(&nextPortForwardRequestID, 1), 10)

			headers := http.Header{}
			headers.Set(corev1.StreamType, corev1.StreamTypeError)
			headers.Set(corev1.PortHeader, strconv.Itoa(podPort))
			headers.Set(corev1.PortForwardRequestIDHeader, id)

			errStream, err := conn.CreateStream(headers)
			if err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("unable to open error stream: %w", err)
			}

			headers.Set(corev1.StreamType, corev1.StreamTypeData)
			dataStream, err := conn.CreateStream(headers)
			if err != nil {
				_ = errStream.Close()
				_ = conn.Close()
				return nil, fmt.Errorf("unable to open data stream: %w", err)
			}

			pfc := &portForwardConnection{
				stream:    dataStream,
				errStream: errStream,
				conn:      conn,
				local:     portForwardAddr{network: network, addr: "127.0.0.1:0"},
				remote:    portForwardAddr{network: network, addr: fmt.Sprintf("pod/%s:%d", podName, podPort)},
			}

			pfc.startErrorStreamMonitor()

			return pfc, nil
		},
	}, nil
}

type portForwardConnection struct {
	stream        httpstream.Stream
	errStream     httpstream.Stream
	conn          httpstream.Connection
	local         net.Addr
	remote        net.Addr
	mu            sync.Mutex
	closed        bool
	readDeadline  time.Time
	writeDeadline time.Time
	wg            sync.WaitGroup
}

func (pfc *portForwardConnection) Read(bytes []byte) (n int, err error) {
	pfc.mu.Lock()
	deadline := pfc.readDeadline
	pfc.mu.Unlock()

	return pfc.executeWithDeadline(deadline, func() (int, error) {
		return pfc.stream.Read(bytes)
	})
}

func (pfc *portForwardConnection) Write(bytes []byte) (n int, err error) {
	pfc.mu.Lock()
	deadline := pfc.writeDeadline
	pfc.mu.Unlock()

	return pfc.executeWithDeadline(deadline, func() (int, error) {
		return pfc.stream.Write(bytes)
	})
}

func (pfc *portForwardConnection) Close() error {
	pfc.mu.Lock()
	if pfc.closed {
		pfc.mu.Unlock()
		return nil
	}
	pfc.closed = true
	pfc.mu.Unlock()

	_ = pfc.stream.Close()
	_ = pfc.errStream.Close()
	pfc.wg.Wait()
	return pfc.conn.Close()
}

func (pfc *portForwardConnection) LocalAddr() net.Addr {
	return pfc.local
}

func (pfc *portForwardConnection) RemoteAddr() net.Addr {
	return pfc.remote
}

func (pfc *portForwardConnection) SetDeadline(t time.Time) error {
	pfc.mu.Lock()
	defer pfc.mu.Unlock()
	pfc.readDeadline = t
	pfc.writeDeadline = t
	return nil
}

func (pfc *portForwardConnection) SetReadDeadline(t time.Time) error {
	pfc.mu.Lock()
	defer pfc.mu.Unlock()
	pfc.readDeadline = t
	return nil
}

func (pfc *portForwardConnection) SetWriteDeadline(t time.Time) error {
	pfc.mu.Lock()
	defer pfc.mu.Unlock()
	pfc.writeDeadline = t
	return nil
}

func (pfc *portForwardConnection) executeWithDeadline(deadline time.Time, operation func() (int, error)) (int, error) {
	if deadline.IsZero() {
		return operation()
	}

	if time.Now().After(deadline) {
		return 0, os.ErrDeadlineExceeded
	}

	type result struct {
		n   int
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		n, err := operation()
		resultCh <- result{n, err}
	}()

	select {
	case res := <-resultCh:
		return res.n, res.err
	case <-time.After(time.Until(deadline)):
		return 0, os.ErrDeadlineExceeded
	}
}

func (pfc *portForwardConnection) startErrorStreamMonitor() {
	pfc.wg.Add(1)
	go func() {
		defer pfc.wg.Done()
		msg, readErr := io.ReadAll(pfc.errStream)
		switch {
		case readErr != nil && readErr != io.EOF:
			_, _ = fmt.Fprintf(os.Stderr, "port-forward: error reading error stream: %v\n", readErr)
		case len(msg) > 0:
			_, _ = fmt.Fprintf(os.Stderr, "port-forward error: %s\n", string(msg))
		}
	}()
}

type portForwardAddr struct {
	network string
	addr    string
}

func (a portForwardAddr) Network() string { return a.network }
func (a portForwardAddr) String() string  { return a.addr }
