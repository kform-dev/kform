package grabber

import (
	"hash"
	"net/http"
	"net/url"
)

// A Hook is a user provided callback function that can be called by grab at
// various stages of a requests lifecycle. If a hook returns an error, the
// associated request is canceled and the same error is returned on the Response
// object.
//
// Hook functions are called synchronously and should never block unnecessarily.
// Response methods that block until a download is complete, such as
// Response.Err, Response.Cancel or Response.Wait will deadlock. To cancel a
// download from a callback, simply return a non-nil error.
type Hook func(*Response) error

type Request struct {
	// HTTPRequest specifies the http.Request to be sent to the remote server to
	// initiate a file transfer. It includes request configuration such as URL,
	// protocol version, HTTP method, request headers and authentication.
	HTTPRequest *http.Request

	// NoResume specifies that a partially completed download will be restarted
	// without attempting to resume any existing file. If the download is already
	// completed in full, it will not be restarted.
	NoResume bool

	// Path specifies the path where the file transfer will be stored in
	// local storage. If Path is empty or a directory, the true Filename will
	// be resolved using Content-Disposition headers or the request URL.
	//
	// An empty string means the transfer will be stored in the current working
	// directory.
	Filename string

	// IgnoreBadStatusCodes specifies that grab should accept any status code in
	// the response from the remote server. Otherwise, grab expects the response
	// status code to be within the 2XX range (after following redirects).
	IgnoreBadStatusCodes bool

	// IgnoreRemoteTime specifies that grab should not attempt to set the
	// timestamp of the local file to match the remote file.
	IgnoreRemoteTime bool

	// Size specifies the expected size of the file transfer if known. If the
	// server response size does not match, the transfer is cancelled and
	// ErrBadLength returned.
	Size int64

	// BufferSize specifies the size in bytes of the buffer that is used for
	// transferring the requested file. Larger buffers may result in faster
	// throughput but will use more memory and result in less frequent updates
	// to the transfer progress statistics. If a RateLimiter is configured,
	// BufferSize should be much lower than the rate limit. Default: 32KB.
	BufferSize int

	// RateLimiter allows the transfer rate of a download to be limited. The given
	// Request.BufferSize determines how frequently the RateLimiter will be
	// polled.
	RateLimiter RateLimiter

	// BeforeCopy is a user provided callback that is called immediately before
	// a request starts downloading. If BeforeCopy returns an error, the request
	// is cancelled and the same error is returned on the Response object.
	BeforeCopy Hook

	// AfterCopy is a user provided callback that is called immediately after a
	// request has finished downloading, before checksum validation and closure.
	// This hook is only called if the transfer was successful. If AfterCopy
	// returns an error, the request is canceled and the same error is returned on
	// the Response object.
	AfterCopy Hook

	// hash, checksum and deleteOnError - set via SetChecksum.
	hash          hash.Hash
	checksum      []byte
	deleteOnError bool
}

// NewRequest returns a new file transfer Request suitable for use with
// Client.Do.
func NewRequest(path, url string) (*Request, error) {
	if path == "" {
		path = "."
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return &Request{
		HTTPRequest: req,
		Filename:    path,
	}, nil
}

// URL returns the URL to be downloaded.
func (r *Request) URL() *url.URL {
	return r.HTTPRequest.URL
}
