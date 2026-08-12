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

func DefaultModifyResponse(onComplete OnCompleteFunc, transforms ...StreamTransformFunc) ModifyResponseFunc {
	var streamTransform StreamTransformFunc
	if len(transforms) > 0 {
		streamTransform = transforms[0]
	}
	return func(resp *http.Response) error {
		if resp == nil || resp.Body == nil {
			return nil
		}

		if isStreamingResponse(resp) {
			if streamTransform != nil {
				processor, ok, err := streamTransform(resp)
				if err != nil {
					return err
				}
				if ok && processor != nil {
					resp.Body = newTransformingReadCloser(resp, resp.Body, processor, onComplete)
					resp.ContentLength = -1
					resp.Header.Del("Content-Length")
					return nil
				}
			}
			if onComplete == nil {
				return nil
			}
			resp.Body = newCompletionReadCloser(resp, resp.Body, onComplete)
			resp.ContentLength = -1
			resp.Header.Del("Content-Length")
			return nil
		}
		if !shouldCaptureResponseBody(resp) {
			onComplete(resp, nil)
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

type transformingReadCloser struct {
	resp       *http.Response
	body       io.ReadCloser
	processor  StreamProcessor
	onComplete OnCompleteFunc
	raw        bytes.Buffer
	out        bytes.Buffer
	pendingErr error
	finished   bool
	stopped    bool
	once       sync.Once
	closeOnce  sync.Once
}

func newTransformingReadCloser(resp *http.Response, body io.ReadCloser, processor StreamProcessor, onComplete OnCompleteFunc) *transformingReadCloser {
	return &transformingReadCloser{resp: resp, body: body, processor: processor, onComplete: onComplete}
}

func (r *transformingReadCloser) Read(p []byte) (int, error) {
	if r.out.Len() == 0 && r.pendingErr != nil {
		err := r.pendingErr
		r.pendingErr = nil
		return 0, err
	}
	for r.out.Len() == 0 && !r.finished {
		buf := make([]byte, len(p))
		if len(buf) == 0 {
			buf = make([]byte, 32*1024)
		}
		n, err := r.body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = r.raw.Write(chunk)
			result, writeErr := r.processor.Write(chunk)
			if len(result.Data) > 0 {
				_, _ = r.out.Write(result.Data)
			}
			if result.Stop {
				r.stopped = true
				r.finished = true
				r.closeBodyAndProcessor()
				break
			}
			if writeErr != nil {
				r.finished = true
				r.pendingErr = writeErr
				break
			}
		}
		if err == io.EOF {
			finishResult, finishErr := r.processor.Finish()
			if len(finishResult.Data) > 0 {
				_, _ = r.out.Write(finishResult.Data)
			}
			r.finished = true
			if finishResult.Stop {
				r.stopped = true
				r.closeBodyAndProcessor()
			} else {
				r.complete()
				r.closeProcessor()
			}
			if finishErr != nil {
				r.pendingErr = finishErr
			}
			break
		}
		if err != nil {
			r.finished = true
			r.pendingErr = err
			break
		}
	}
	if r.out.Len() > 0 {
		return r.out.Read(p)
	}
	if r.pendingErr != nil {
		err := r.pendingErr
		r.pendingErr = nil
		return 0, err
	}
	return 0, io.EOF
}

func (r *transformingReadCloser) Close() error {
	err := r.body.Close()
	r.closeProcessor()
	if !r.stopped {
		r.complete()
	}
	return err
}

func (r *transformingReadCloser) complete() {
	if r.onComplete == nil {
		return
	}
	r.once.Do(func() {
		decodedBody, err := DecodeResponseBody(r.raw.Bytes(), r.resp.Header.Get("Content-Encoding"))
		if err != nil {
			decodedBody = nil
		}
		r.onComplete(r.resp, decodedBody)
	})
}

func (r *transformingReadCloser) closeBodyAndProcessor() {
	_ = r.body.Close()
	r.closeProcessor()
}

func (r *transformingReadCloser) closeProcessor() {
	r.closeOnce.Do(func() {
		_ = r.processor.Close()
	})
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

func shouldCaptureResponseBody(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Disposition")), "attachment") {
		return false
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	mediaType := strings.TrimSpace(strings.Split(contentType, ";")[0])
	if mediaType == "" {
		return true
	}
	if strings.HasPrefix(mediaType, "video/") || strings.HasPrefix(mediaType, "audio/") || strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "font/") {
		return false
	}

	switch mediaType {
	case "application/octet-stream",
		"application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-tar",
		"application/x-7z-compressed",
		"application/x-rar-compressed",
		"application/wasm":
		return false
	}

	return true
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
