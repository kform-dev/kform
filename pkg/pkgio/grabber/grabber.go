package grabber

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GetBatch sends multiple HTTP requests and downloads the content of the
// requested URLs to the given destination directory using the given number of
// concurrent worker goroutines.
//
// fileLocs is a struct which contains a destination as a directory where multiple
// urls can write to
//
// The Response for each requested URL is sent through the returned Response
// channel, as soon as a worker receives a response from the remote server. The
// Response can then be used to track the progress of the download while it is
// in progress.
//
// The returned Response channel will be closed, only once all downloads
// have completed or failed.
//
// If an error occurs during any download, it will be available via call to the
// associated Response.Err.
//
// For control over HTTP client headers, redirect policy, and other settings,
// create a Client instead.
func GetBatch(ctx context.Context, workers int, fileLocs map[string][]string) (<-chan *Response, error) {
	for path := range fileLocs {
		fi, err := os.Stat(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("filepath is not a directory")
		}
	}

	reqs := make([]*Request, 0, GetTotalURLs(fileLocs))
	for path, urls := range fileLocs {
		for _, url := range urls {
			req, err := NewRequest(path, url)
			if err != nil {
				return nil, err
			}
			reqs = append(reqs, req)
		}
	}

	ch := NewClient().DoBatch(ctx, workers, reqs...)
	return ch, nil
}

func GetTotalURLs(fileLocs map[string][]string) int {
	total := 0
	for _, urls := range fileLocs {
		total += len(urls)
	}
	return total
}

// Do sends a file transfer request and returns a file transfer response,
// following policy (e.g. redirects, cookies, auth) as configured on the
// client's HTTPClient.
//
// Like http.Get, Do blocks while the transfer is initiated, but returns as soon
// as the transfer has started transferring in a background goroutine, or if it
// failed early.
//
// An error is returned via Response.Err if caused by client policy (such as
// CheckRedirect), or if there was an HTTP protocol or IO error. Response.Err
// will block the caller until the transfer is completed, successfully or
// otherwise.
func (r *Client) Do(ctx context.Context, req *Request) *Response {
	// cancel will be called on all code-paths via closeResponse
	ctx, cancel := context.WithCancel(ctx)
	resp := &Response{
		Request:    req,
		Start:      time.Now(),
		Done:       make(chan struct{}),
		Filename:   req.Filename,
		cancel:     cancel,
		bufferSize: req.BufferSize,
	}
	if resp.bufferSize == 0 {
		// default to Client.BufferSize
		resp.bufferSize = r.BufferSize
	}

	// Run state-machine while caller is blocked to initialize the file transfer.
	// Must never transition to the copyFile state - this happens next in another
	// goroutine.
	r.run(ctx, resp, r.statFileInfo)

	// Run copyFile in a new goroutine. copyFile will no-op if the transfer is
	// already complete or failed.
	go r.run(ctx, resp, r.copyFile)
	return resp
}

// DoChannel executes all requests sent through the given Request channel, one
// at a time, until it is closed by another goroutine. The caller is blocked
// until the Request channel is closed and all transfers have completed. All
// responses are sent through the given Response channel as soon as they are
// received from the remote servers and can be used to track the progress of
// each download.
//
// Slow Response receivers will cause a worker to block and therefore delay the
// start of the transfer for an already initiated connection - potentially
// causing a server timeout. It is the caller's responsibility to ensure a
// sufficient buffer size is used for the Response channel to prevent this.
//
// If an error occurs during any of the file transfers it will be accessible via
// the associated Response.Err function.
func (r *Client) DoChannel(ctx context.Context, reqch <-chan *Request, respch chan<- *Response) {
	// TODO: enable cancelling of batch jobs
	for req := range reqch {
		resp := r.Do(ctx, req)
		respch <- resp
		<-resp.Done
	}
}

// DoBatch executes all the given requests using the given number of concurrent
// workers. Control is passed back to the caller as soon as the workers are
// initiated.
//
// If the requested number of workers is less than one, a worker will be created
// for every request. I.e. all requests will be executed concurrently.
//
// If an error occurs during any of the file transfers it will be accessible via
// call to the associated Response.Err.
//
// The returned Response channel is closed only after all of the given Requests
// have completed, successfully or otherwise.
func (r *Client) DoBatch(ctx context.Context, workers int, requests ...*Request) <-chan *Response {
	if workers < 1 {
		workers = len(requests)
	}
	reqch := make(chan *Request, len(requests))
	respch := make(chan *Response, len(requests))
	wg := sync.WaitGroup{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			r.DoChannel(ctx, reqch, respch)
			wg.Done()
		}()
	}

	// queue requests
	go func() {
		for _, req := range requests {
			reqch <- req
		}
		close(reqch)
		wg.Wait()
		close(respch)
	}()
	return respch
}
