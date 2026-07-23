package k8s

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

// blockedStream blocks Read and Write until unblock is closed, then delivers
// payload. It stands in for a pod stream that answers after a deadline fired.
type blockedStream struct {
	unblock chan struct{}
	payload []byte
}

func (s *blockedStream) Read(p []byte) (int, error)  { <-s.unblock; return copy(p, s.payload), nil }
func (s *blockedStream) Write(p []byte) (int, error) { <-s.unblock; return len(p), nil }
func (s *blockedStream) Close() error                { return nil }
func (s *blockedStream) Reset() error                { return nil }
func (s *blockedStream) Headers() http.Header        { return nil }
func (s *blockedStream) Identifier() uint32          { return 0 }

// A Read that times out leaks a worker still blocked on the stream. When that
// worker finally gets data, it must not scribble into the caller's buffer,
// which the caller is free to reuse after the timeout.
func TestReadDeadlineLeavesCallerBufferUntouched(t *testing.T) {
	stream := &blockedStream{unblock: make(chan struct{}), payload: []byte("late data")}
	pfc := &portForwardConnection{stream: stream, errDone: make(chan struct{})}
	close(pfc.errDone) // no error-stream monitor running in this test

	_ = pfc.SetReadDeadline(time.Now().Add(20 * time.Millisecond))

	buf := make([]byte, 16)
	n, err := pfc.Read(buf)
	if n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Read() = (%d, %v), want (0, os.ErrDeadlineExceeded)", n, err)
	}

	close(stream.unblock)
	time.Sleep(50 * time.Millisecond) // give the leaked worker time to finish its read
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("caller's buffer modified at index %d after timeout: %q", i, buf)
		}
	}
}
