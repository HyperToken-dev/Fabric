package proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

const eventStreamContentType = "text/event-stream"

func DefaultModifyResponse(onComplete OnCompleteFunc) ModifyResponseFunc {
	return func(resp *http.Response) error {
		if onComplete == nil || resp == nil || resp.Body == nil {
			return nil
		}

		if isStreamingResponse(resp) {
			resp.Body = newCompletionReadCloser(resp, resp.Body, onComplete)
			resp.ContentLength = -1
			resp.Header.Del("Content-Length")
			return nil
		}

		rawBody, err := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if err != nil {
			if closeErr != nil {
				return fmt.Errorf("read response body: %w; close response body: %v", err, closeErr)
			}
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		decodedBody, decodeErr := DecodeResponseBody(rawBody, resp.Header.Get("Content-Encoding"))
		if decodeErr != nil {
			decodedBody = nil
		}
		restoreResponseBody(resp, rawBody)
		onComplete(resp, decodedBody)
		return nil
	}
}

func DecodeResponseBody(rawBody []byte, contentEncoding string) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return rawBody, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(rawBody))
		if err != nil {
			return nil, err
		}
		decoded, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return decoded, nil
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(rawBody)))
	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", contentEncoding)
	}
}

func isStreamingResponse(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), eventStreamContentType)
}

func restoreResponseBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
}

type completionReadCloser struct {
	resp       *http.Response
	body       io.ReadCloser
	onComplete OnCompleteFunc
	raw        bytes.Buffer
	once       sync.Once
}

func newCompletionReadCloser(resp *http.Response, body io.ReadCloser, onComplete OnCompleteFunc) *completionReadCloser {
	return &completionReadCloser{resp: resp, body: body, onComplete: onComplete}
}

func (r *completionReadCloser) Read(p []byte) (int, error) {
	n, err := r.body.Read(p)
	if n > 0 {
		_, _ = r.raw.Write(p[:n])
	}
	if err == io.EOF {
		r.complete()
	}
	return n, err
}

func (r *completionReadCloser) Close() error {
	err := r.body.Close()
	r.complete()
	return err
}

func (r *completionReadCloser) complete() {
	r.once.Do(func() {
		decodedBody, err := DecodeResponseBody(r.raw.Bytes(), r.resp.Header.Get("Content-Encoding"))
		if err != nil {
			decodedBody = nil
		}
		r.onComplete(r.resp, decodedBody)
	})
}
