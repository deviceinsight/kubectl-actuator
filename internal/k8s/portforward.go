package k8s

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/transport/spdy"
	"net"
	"net/http"
	"strconv"
	"time"
)

func (c Connection) CreateHttpTransport(podName string, podPort int) (*http.Transport, error) {
	portforwardUrl := c.RestClient.Post().
		Resource("pods").
		Namespace(c.Namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.RestConfig)
	if err != nil {
		return nil, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", portforwardUrl)
	connection, _, err := dialer.Dial("portforward.k8s.io")
	if err != nil {
		return nil, errors.Wrap(err, "Unable to dial portforward protocol")
	}

	headers := http.Header{}
	headers.Set(corev1.StreamType, corev1.StreamTypeError)
	headers.Set(corev1.PortHeader, strconv.Itoa(podPort))
	headers.Set(corev1.PortForwardRequestIDHeader, "1") // XXX: Should this always be 1?

	errorStream, err := connection.CreateStream(headers)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to open data stream")
	}
	err = errorStream.Close()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to close error stream")
	}

	// XXX: The errors shouldn't be just printed to stdout
	go func() {
		message, err := io.ReadAll(errorStream)
		switch {
		case err != nil:
			fmt.Println("Error reading error")
		case len(message) > 0:
			fmt.Println("Error: ", string(message))
		}
	}()

	headers.Set(corev1.StreamType, corev1.StreamTypeData)
	dataStream, err := connection.CreateStream(headers)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to open data stream")
	}

	return &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return portForwardConnection{stream: dataStream}, nil
		},
	}, nil
}

type portForwardConnection struct {
	stream httpstream.Stream
}

func (transport portForwardConnection) Read(bytes []byte) (n int, err error) {
	return transport.stream.Read(bytes)
}

func (transport portForwardConnection) Write(bytes []byte) (n int, err error) {
	return transport.stream.Write(bytes)
}

func (transport portForwardConnection) Close() error {
	return transport.stream.Close()
}

func (transport portForwardConnection) LocalAddr() net.Addr {
	panic("not implemented")
}

func (transport portForwardConnection) RemoteAddr() net.Addr {
	panic("not implemented")
}

func (transport portForwardConnection) SetDeadline(_ time.Time) error {
	panic("not implemented")
}

func (transport portForwardConnection) SetReadDeadline(_ time.Time) error {
	panic("not implemented")
}

func (transport portForwardConnection) SetWriteDeadline(_ time.Time) error {
	panic("not implemented")
}
