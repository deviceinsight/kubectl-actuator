package util

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"net"
	"net/http"
	"time"
)

func CreateHttpTransport(
	pod *corev1.Pod,
	restClient *rest.RESTClient,
	restConfig *rest.Config,
) (*http.Transport, error) {
	// TODO:
	// - Clean up this code
	// - Make port configurable via label
	// - Mak actuator base url configurable via label

	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod is not running. Current status=%v", pod.Status.Phase)
	}

	portforwardUrl := restClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
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
	headers.Set(corev1.PortHeader, "9090")              // TODO
	headers.Set(corev1.PortForwardRequestIDHeader, "1") // TODO

	errorStream, err := connection.CreateStream(headers)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to open data stream")
	}
	err = errorStream.Close()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to close error stream")
	}

	go func() {
		message, err := ioutil.ReadAll(errorStream)
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
