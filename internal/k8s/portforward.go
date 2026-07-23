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

func (c *Connection) CreateHTTPTransport(podName string, podPort int) (*http.Transport, error) {
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

	dialer := &podDialer{
		spdy:    spdy.NewDialer(upgrader, &http.Client{Transport: baseTransport}, "POST", portForwardURL),
		podName: podName,
		podPort: podPort,
	}
	return &http.Transport{
		DisableKeepAlives: true,
		DialContext:       dialer.DialContext,
	}, nil
}

// podDialer opens one port-forward connection per dial: an SPDY session
// carrying an error stream and a data stream, wrapped as a net.Conn.
type podDialer struct {
	spdy    httpstream.Dialer
	podName string
	podPort int
}

func (d *podDialer) DialContext(_ context.Context, network, _ string) (net.Conn, error) {
	conn, _, err := d.spdy.Dial("portforward.k8s.io")
	if err != nil {
		return nil, fmt.Errorf("unable to dial portforward protocol: %w", err)
	}

	id := strconv.FormatUint(atomic.AddUint64(&nextPortForwardRequestID, 1), 10)

	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, strconv.Itoa(d.podPort))
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
		// There is no local socket - the net.Conn is synthesized over SPDY
		// streams - so LocalAddr reports a loopback placeholder.
		local:   portForwardAddr{network: network, addr: "127.0.0.1:0"},
		remote:  portForwardAddr{network: network, addr: fmt.Sprintf("pod/%s:%d", d.podName, d.podPort)},
		errDone: make(chan struct{}),
	}

	pfc.startErrorStreamMonitor()

	return pfc, nil
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
	bytesRead     atomic.Int64
	errMsg        string
	errDone       chan struct{}
}

func (pfc *portForwardConnection) Read(bytes []byte) (n int, err error) {
	pfc.mu.Lock()
	deadline := pfc.readDeadline
	pfc.mu.Unlock()

	if deadline.IsZero() {
		n, err = pfc.stream.Read(bytes)
	} else {
		// A timed-out worker keeps reading (see executeWithDeadline), so it
		// gets a buffer of its own; bytes is only written while the caller is
		// still waiting here.
		buf := make([]byte, len(bytes))
		n, err = pfc.executeWithDeadline(deadline, func() (int, error) {
			return pfc.stream.Read(buf)
		})
		copy(bytes, buf[:n])
	}
	if n > 0 {
		pfc.bytesRead.Add(int64(n))
	}
	// A connection that errors before delivering a single byte was most
	// likely rejected pod-side; the kubelet reports the real cause on the
	// error stream while the data stream just closes.
	if err != nil && pfc.bytesRead.Load() == 0 {
		if msg := pfc.portForwardError(); msg != "" {
			return n, fmt.Errorf("%s", msg)
		}
	}
	return n, err
}

func (pfc *portForwardConnection) Write(bytes []byte) (n int, err error) {
	pfc.mu.Lock()
	deadline := pfc.writeDeadline
	pfc.mu.Unlock()

	if deadline.IsZero() {
		return pfc.stream.Write(bytes)
	}
	// A timed-out worker keeps writing (see executeWithDeadline), so it gets a
	// copy; the caller is free to reuse bytes as soon as this returns.
	buf := append([]byte(nil), bytes...)
	return pfc.executeWithDeadline(deadline, func() (int, error) {
		return pfc.stream.Write(buf)
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

// executeWithDeadline runs operation, giving up when the deadline passes. On
// timeout the worker goroutine stays blocked in operation until the stream
// closes. This is an accepted leak, since a timed-out invocation exits shortly
// after; because the worker can outlive the call, operation must only touch
// buffers it owns, never the caller's.
func (pfc *portForwardConnection) executeWithDeadline(deadline time.Time, operation func() (int, error)) (int, error) {
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

// startErrorStreamMonitor captures the kubelet's port-forward error message,
// if any, so the failing Read can return it as the actual cause instead of a
// bare EOF racing a stderr line.
func (pfc *portForwardConnection) startErrorStreamMonitor() {
	pfc.wg.Add(1)
	go func() {
		defer pfc.wg.Done()
		defer close(pfc.errDone)
		msg, readErr := io.ReadAll(pfc.errStream)
		if readErr == nil {
			pfc.mu.Lock()
			pfc.errMsg = string(msg)
			pfc.mu.Unlock()
		}
	}()
}

// portForwardError returns the error-stream message, waiting briefly for the
// monitor since the kubelet writes it at about the same time the data stream
// closes. Only called on a failed connection, never on the happy path.
func (pfc *portForwardConnection) portForwardError() string {
	select {
	case <-pfc.errDone:
	case <-time.After(500 * time.Millisecond):
	}
	pfc.mu.Lock()
	defer pfc.mu.Unlock()
	return pfc.errMsg
}

type portForwardAddr struct {
	network string
	addr    string
}

func (a portForwardAddr) Network() string { return a.network }
func (a portForwardAddr) String() string  { return a.addr }
