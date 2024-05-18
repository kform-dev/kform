package grabber

import (
	"context"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Response represents the response to a download request.
//
// A response may be returned as soon a HTTP response is received from a remote
// server, but before the body content has started transferring.
//
// All Response method calls are thread-safe
type Response struct {
	// The Request that was submitted to obtain this Response.
	Request *Request

	// HTTPResponse represents the HTTP response received from an HTTP request.
	//
	// The response Body should not be used as it will be consumed and closed by
	// the grabber.
	HTTPResponse *http.Response

	// Path specifies the path where the file transfer is stored in local
	// storage.
	Filename string

	// Size specifies the total expected size of the file transfer.
	Size int64

	// Start specifies the time at which the file transfer started.
	Start time.Time

	// End specifies the time at which the file transfer completed.
	//
	// This will return zero until the transfer has completed.
	End time.Time

	// CanResume specifies that the remote server advertised that it can resume
	// previous downloads, as the 'Accept-Ranges: bytes' header is set.
	CanResume bool

	// DidResume specifies that the file transfer resumed a previously incomplete
	// transfer.
	DidResume bool

	// Done is closed once the transfer is finalized, either successfully or with
	// errors. Errors are available via Response.Err
	Done chan struct{}

	// cancel is a cancel func that can be used to cancel the context of this
	// Response.
	cancel context.CancelFunc

	// optionsKnown indicates that a HEAD request has been completed and the
	// capabilities of the remote server are known.
	optionsKnown bool

	// fi is the FileInfo for the destination file if it already existed before
	// transfer started.
	fi os.FileInfo

	// writer is the file handle used to write the downloaded file to local
	// storage
	writer io.WriteCloser

	// bytesCompleted specifies the number of bytes which were already
	// transferred before this transfer began.
	bytesResumed int64

	// transfer is responsible for copying data from the remote server to a local
	// file, tracking progress and allowing for cancelation.
	transfer *transfer

	// bytesPerSecond specifies the number of bytes that have been transferred in
	// the last 1-second window.
	bytesPerSecond float64
	m              sync.Mutex

	// bufferSize specifies the size in bytes of the transfer buffer.
	bufferSize int

	// Error contains any error that may have occurred during the file transfer.
	// This should not be read until IsComplete returns true.
	err error
}

// IsComplete returns true if the download has completed. If an error occurred
// during the download, it can be returned via Err.
func (r *Response) IsComplete() bool {
	select {
	case <-r.Done:
		return true
	default:
		return false
	}
}

// Err blocks the calling goroutine until the underlying file transfer is
// completed and returns any error that may have occurred. If the download is
// already completed, Err returns immediately.
func (c *Response) Err() error {
	<-c.Done
	return c.err
}

// BytesComplete returns the total number of bytes which have been copied to
// the destination, including any bytes that were resumed from a previous
// download.
func (c *Response) BytesComplete() int64 {
	return c.bytesResumed + c.transfer.N()
}

// BytesPerSecond returns the number of bytes transferred in the last second. If
// the download is already complete, the average bytes/sec for the life of the
// download is returned.
func (r *Response) BytesPerSecond() float64 {
	if r.IsComplete() {
		return float64(r.transfer.N()) / r.Duration().Seconds()
	}
	r.m.Lock()
	defer r.m.Unlock()
	return r.bytesPerSecond
}

// Progress returns the ratio of total bytes that have been downloaded. Multiply
// the returned value by 100 to return the percentage completed.
func (r *Response) Progress() float64 {
	if r.Size == 0 {
		return 0
	}
	return float64(r.BytesComplete()) / float64(r.Size)
}

// Duration returns the duration of a file transfer. If the transfer is in
// process, the duration will be between now and the start of the transfer. If
// the transfer is complete, the duration will be between the start and end of
// the completed transfer process.
func (c *Response) Duration() time.Duration {
	if c.IsComplete() {
		return c.End.Sub(c.Start)
	}

	return time.Since(c.Start)
}

// ETA returns the estimated time at which the the download will complete, given
// the current BytesPerSecond. If the transfer has already completed, the actual
// end time will be returned.
func (r *Response) ETA() time.Time {
	if r.IsComplete() {
		return r.End
	}
	bt := r.BytesComplete()
	bps := r.BytesPerSecond()
	if bps == 0 {
		return time.Time{}
	}
	secs := float64(r.Size-bt) / bps
	return time.Now().Add(time.Duration(secs) * time.Second)
}

// watchBps watches the progress of a transfer and maintains statistics.
func (r *Response) watchBps() {
	var prev int64
	then := r.Start

	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-r.Done:
			return

		case now := <-t.C:
			d := now.Sub(then)
			then = now

			cur := r.transfer.N()
			bs := cur - prev
			prev = cur

			r.m.Lock()
			r.bytesPerSecond = float64(bs) / d.Seconds()
			r.m.Unlock()
		}
	}
}

func (r *Response) requestMethod() string {
	if r == nil || r.HTTPResponse == nil || r.HTTPResponse.Request == nil {
		return ""
	}
	return r.HTTPResponse.Request.Method
}

func (r *Response) closeResponseBody() error {
	if r.HTTPResponse == nil || r.HTTPResponse.Body == nil {
		return nil
	}
	return r.HTTPResponse.Body.Close()
}
