package k8s

import (
	"context"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/transport/spdy"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var portForwardReqID uint64

func (c *Connection) CreateHttpTransport(podName string, podPort int) (*http.Transport, error) {
	portforwardUrl := c.restClient.Post().
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
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: baseTransport}, "POST", portforwardUrl)
			conn, _, err := dialer.Dial("portforward.k8s.io")
			if err != nil {
				return nil, fmt.Errorf("unable to dial portforward protocol: %w", err)
			}

			id := strconv.FormatUint(atomic.AddUint64(&portForwardReqID, 1), 10)

			headers := http.Header{}
			headers.Set(corev1.StreamType, corev1.StreamTypeError)
			headers.Set(corev1.PortHeader, strconv.Itoa(podPort))
			headers.Set(corev1.PortForwardRequestIDHeader, id)

			errStream, err := conn.CreateStream(headers)
			if err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("unable to open error stream: %w", err)
			}

			// Drain error stream in background; print to stderr if any content
			go func() {
				msg, readErr := io.ReadAll(errStream)
				switch {
				case readErr != nil && readErr != io.EOF:
					_, _ = fmt.Fprintf(os.Stderr, "port-forward: error reading error stream: %v\n", readErr)
				case len(msg) > 0:
					_, _ = fmt.Fprintf(os.Stderr, "port-forward error: %s\n", string(msg))
				}
				_ = errStream.Close()
			}()

			headers.Set(corev1.StreamType, corev1.StreamTypeData)
			dataStream, err := conn.CreateStream(headers)
			if err != nil {
				_ = errStream.Close()
				_ = conn.Close()
				return nil, fmt.Errorf("unable to open data stream: %w", err)
			}

			return &portForwardConnection{
				stream:    dataStream,
				errStream: errStream,
				conn:      conn,
				local:     dummyAddr{network: network, addr: "127.0.0.1:0"},
				remote:    dummyAddr{network: network, addr: fmt.Sprintf("pod/%s:%d", podName, podPort)},
			}, nil
		},
	}, nil
}

type portForwardConnection struct {
	stream    httpstream.Stream
	errStream httpstream.Stream
	conn      httpstream.Connection
	local     net.Addr
	remote    net.Addr
	mu        sync.Mutex
	closed    bool
}

func (transport *portForwardConnection) Read(bytes []byte) (n int, err error) {
	return transport.stream.Read(bytes)
}

func (transport *portForwardConnection) Write(bytes []byte) (n int, err error) {
	return transport.stream.Write(bytes)
}

func (transport *portForwardConnection) Close() error {
	transport.mu.Lock()
	if transport.closed {
		transport.mu.Unlock()
		return nil
	}
	transport.closed = true
	transport.mu.Unlock()

	_ = transport.stream.Close()
	_ = transport.errStream.Close()
	return transport.conn.Close()
}

func (transport *portForwardConnection) LocalAddr() net.Addr {
	return transport.local
}

func (transport *portForwardConnection) RemoteAddr() net.Addr {
	return transport.remote
}

func (transport *portForwardConnection) SetDeadline(_ time.Time) error {
	return nil
}

func (transport *portForwardConnection) SetReadDeadline(_ time.Time) error {
	return nil
}

func (transport *portForwardConnection) SetWriteDeadline(_ time.Time) error {
	return nil
}

type dummyAddr struct {
	network string
	addr    string
}

func (a dummyAddr) Network() string { return a.network }
func (a dummyAddr) String() string  { return a.addr }
