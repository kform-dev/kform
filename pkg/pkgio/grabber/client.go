package grabber

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// A Client is a file download client.
//
// Clients are safe for concurrent use by multiple goroutines.
type Client struct {
	// HTTPClient specifies the http.Client which will be used for communicating
	// with the remote server during the file transfer.
	HTTPClient *http.Client

	// UserAgent specifies the User-Agent string which will be set in the
	// headers of all requests made by this client.
	//
	// The user agent string may be overridden in the headers of each request.
	UserAgent string

	// BufferSize specifies the size in bytes of the buffer that is used for
	// transferring all requested files. Larger buffers may result in faster
	// throughput but will use more memory and result in less frequent updates
	// to the transfer progress statistics. The BufferSize of each request can
	// be overridden on each Request object. Default: 32KB.
	BufferSize int
}

// NewClient returns a new file download Client, using default configuration.
func NewClient() *Client {
	return &Client{
		UserAgent: "kform",
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

// An stateFunc is an action that mutates the state of a Response and returns
// the next stateFunc to be called.
type stateFunc func(context.Context, *Response) stateFunc

// run calls the given stateFunc function and all subsequent returned stateFuncs
// until a stateFunc returns nil or the Response.ctx is canceled. Each stateFunc
// should mutate the state of the given Response until it has completed
// downloading or failed.
func (r *Client) run(ctx context.Context, resp *Response, f stateFunc) {
	for {
		select {
		case <-ctx.Done():
			if resp.IsComplete() {
				return
			}
			resp.err = ctx.Err()
			f = r.closeResponse

		default:
			// keep working
		}
		if f = f(ctx, resp); f == nil {
			return
		}
	}
}

// statFileInfo retrieves FileInfo for any local file matching
// Response.Filename.
//
// If the file does not exist, is a directory, or its name is unknown the next
// stateFunc is headRequest.
//
// If the file exists, Response.fi is set and the next stateFunc is
// validateLocal.
//
// If an error occurs, the next stateFunc is closeResponse.
func (r *Client) statFileInfo(ctx context.Context, resp *Response) stateFunc {
	if resp.Filename == "" {
		return r.headRequest
	}
	fi, err := os.Stat(filepath.Dir(resp.Filename))
	if err != nil {
		if os.IsNotExist(err) {
			return r.headRequest
		}
		resp.err = err
		return r.closeResponse
	}
	if fi.IsDir() {
		resp.Filename = ""
		return r.headRequest
	}
	resp.fi = fi
	return r.validateLocal
}

// validateLocal compares a local copy of the downloaded file to the remote
// file.
//
// An error is returned if the local file is larger than the remote file, or
// Request.SkipExisting is true.
//
// If the existing file matches the length of the remote file, the next
// stateFunc is checksumFile.
//
// If the local file is smaller than the remote file and the remote server is
// known to support ranged requests, the next stateFunc is getRequest.
func (r *Client) validateLocal(ctx context.Context, resp *Response) stateFunc {
	// determine expected file size
	size := resp.Request.Size
	if size == 0 && resp.HTTPResponse != nil {
		size = resp.HTTPResponse.ContentLength
	}
	if size == 0 {
		return r.headRequest
	}

	if size == resp.fi.Size() {
		resp.DidResume = true
		resp.bytesResumed = resp.fi.Size()
		return r.checksumFile
	}

	if resp.Request.NoResume {
		return r.getRequest
	}

	if size < resp.fi.Size() {
		resp.err = ErrBadLength
		return r.closeResponse
	}

	if resp.CanResume {
		resp.Request.HTTPRequest.Header.Set(
			"Range",
			fmt.Sprintf("bytes=%d-", resp.fi.Size()))
		resp.DidResume = true
		resp.bytesResumed = resp.fi.Size()
		return r.getRequest
	}
	return r.headRequest
}

func (r *Client) checksumFile(ctx context.Context, resp *Response) stateFunc {
	if resp.Request.hash == nil {
		return r.closeResponse
	}
	if resp.Filename == "" {
		panic("filename not set")
	}
	req := resp.Request

	// compare checksum
	var sum []byte
	sum, resp.err = checksum(ctx, resp.Filename, req.hash)
	if resp.err != nil {
		return r.closeResponse
	}
	if !bytes.Equal(sum, req.checksum) {
		resp.err = ErrBadChecksum
		if req.deleteOnError {
			if err := os.Remove(resp.Filename); err != nil {
				// err should be os.PathError and include file path
				resp.err = fmt.Errorf(
					"cannot remove downloaded file with checksum mismatch: %v",
					err)
			}
		}
	}
	return r.closeResponse
}

// doHTTPRequest sends a HTTP Request and returns the response
func (c *Client) doHTTPRequest(_ context.Context, req *http.Request) (*http.Response, error) {
	if c.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	return c.HTTPClient.Do(req)
}

func (r *Client) headRequest(ctx context.Context, resp *Response) stateFunc {
	if resp.optionsKnown {
		return r.getRequest
	}
	resp.optionsKnown = true

	if resp.Request.NoResume {
		return r.getRequest
	}

	if resp.Filename != "" && resp.fi == nil {
		// destination path is already known and does not exist
		return r.getRequest
	}

	hreq := new(http.Request)
	*hreq = *resp.Request.HTTPRequest
	hreq.Method = "HEAD"

	resp.HTTPResponse, resp.err = r.doHTTPRequest(ctx, hreq)
	if resp.err != nil {
		return r.closeResponse
	}
	resp.HTTPResponse.Body.Close()

	if resp.HTTPResponse.StatusCode != http.StatusOK {
		return r.getRequest
	}

	return r.readResponse
}

func (r *Client) getRequest(ctx context.Context, resp *Response) stateFunc {
	resp.HTTPResponse, resp.err = r.doHTTPRequest(ctx, resp.Request.HTTPRequest)
	if resp.err != nil {
		return r.closeResponse
	}

	// check status code
	if !resp.Request.IgnoreBadStatusCodes {
		if resp.HTTPResponse.StatusCode < 200 || resp.HTTPResponse.StatusCode > 299 {
			resp.err = StatusCodeError(resp.HTTPResponse.StatusCode)
			return r.closeResponse
		}
	}

	return r.readResponse
}

func (r *Client) readResponse(ctx context.Context, resp *Response) stateFunc {
	if resp.HTTPResponse == nil {
		panic("Response.HTTPResponse is not ready")
	}

	// check expected size
	resp.Size = resp.bytesResumed + resp.HTTPResponse.ContentLength
	if resp.HTTPResponse.ContentLength > 0 && resp.Request.Size > 0 {
		if resp.Request.Size != resp.Size {
			resp.err = ErrBadLength
			return r.closeResponse
		}
	}

	// check filename
	if resp.Filename == "" {
		filename, err := guessFilename(resp.HTTPResponse)
		if err != nil {
			resp.err = err
			return r.closeResponse
		}
		// Request.Filename will be empty or a directory
		resp.Filename = filepath.Join(resp.Request.Filename, filename)
	}

	if resp.requestMethod() == "HEAD" {
		if resp.HTTPResponse.Header.Get("Accept-Ranges") == "bytes" {
			resp.CanResume = true
		}
		return r.statFileInfo
	}
	return r.openWriter
}

// openWriter opens the destination file for writing and seeks to the location
// from whence the file transfer will resume.
//
// Requires that Response.Filename and resp.DidResume are already be set.
func (r *Client) openWriter(ctx context.Context, resp *Response) stateFunc {
	// compute write flags
	flag := os.O_CREATE | os.O_WRONLY
	if resp.fi != nil {
		if resp.DidResume {
			flag = os.O_APPEND | os.O_WRONLY
		} else {
			flag = os.O_TRUNC | os.O_WRONLY
		}
	}

	// open file
	f, err := os.OpenFile(filepath.Dir(resp.Filename), flag, 0755)
	if err != nil {
		fmt.Println("openWriter", err)
		resp.err = err
		return r.closeResponse
	}
	resp.writer = f

	// seek to start or end
	whence := os.SEEK_SET
	if resp.bytesResumed > 0 {
		whence = os.SEEK_END
	}
	_, resp.err = f.Seek(0, whence)
	if resp.err != nil {
		return r.closeResponse
	}

	// init transfer
	if resp.bufferSize < 1 {
		resp.bufferSize = 32 * 1024
	}
	b := make([]byte, resp.bufferSize)
	resp.transfer = newTransfer(
		ctx,
		resp.Request.RateLimiter,
		resp.writer,
		resp.HTTPResponse.Body,
		b)

	// next step is copyFile, but this will be called later in another goroutine
	return nil
}

// copy transfers content for a HTTP connection established via Client.do()
func (c *Client) copyFile(ctx context.Context, resp *Response) stateFunc {
	if resp.IsComplete() {
		return nil
	}

	// run BeforeCopy hook
	if f := resp.Request.BeforeCopy; f != nil {
		resp.err = f(resp)
		if resp.err != nil {
			return c.closeResponse
		}
	}

	if resp.transfer == nil {
		panic("developer error: Response.transfer is not initialized")
	}
	go resp.watchBps()
	_, resp.err = resp.transfer.copy()
	if resp.err != nil {
		return c.closeResponse
	}
	closeWriter(ctx, resp)

	// set timestamp
	if !resp.Request.IgnoreRemoteTime {
		resp.err = setLastModified(resp.HTTPResponse, resp.Filename)
		if resp.err != nil {
			return c.closeResponse
		}
	}

	// run AfterCopy hook
	if f := resp.Request.AfterCopy; f != nil {
		resp.err = f(resp)
		if resp.err != nil {
			return c.closeResponse
		}
	}

	return c.checksumFile
}

func closeWriter(_ context.Context, resp *Response) {
	if resp.writer != nil {
		resp.writer.Close()
		resp.writer = nil
	}
}

// close finalizes the Response
func (r *Client) closeResponse(ctx context.Context, resp *Response) stateFunc {
	if resp.IsComplete() {
		panic("response already closed")
	}

	resp.fi = nil
	closeWriter(ctx, resp)
	resp.closeResponseBody()

	resp.End = time.Now()
	close(resp.Done)
	if resp.cancel != nil {
		resp.cancel()
	}

	return nil
}
