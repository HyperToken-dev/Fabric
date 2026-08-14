package proxy

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

const eventStreamContentType = "text/event-stream"

// DefaultModifyResponse builds the default reverse-proxy response hook.
//
// Non-stream responses are buffered only when they are safe to capture, decoded
// for onComplete, then restored so the client still receives the original body.
// Streaming responses are wrapped so completion callbacks and optional stream
// transforms can observe the response without forcing the whole stream into
// memory.
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
					body, err := newStreamingDecodeReadCloser(resp.Body, resp.Header.Get("Content-Encoding"))
					if err != nil {
						return err
					}
					readCloser, err := newTransformingReadCloser(resp, body, resp.Header.Get("Content-Encoding"), processor, onComplete)
					if err != nil {
						closeErr := body.Close()
						if closeErr != nil {
							return fmt.Errorf("create stream transformer: %w; close decoded body: %v", err, closeErr)
						}
						return err
					}
					resp.Body = readCloser
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
		if onComplete == nil {
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

// transformingReadCloser decodes an upstream streaming body, passes decoded
// chunks through a StreamProcessor, and re-encodes the processor output for the
// downstream client.
//
// Concurrency: net/http reads and closes one response body serially. The mutable
// buffers, encoder, and processor are not safe for concurrent use. onComplete is
// guarded because EOF and Close can both finish the stream lifecycle.
type transformingReadCloser struct {
	resp       *http.Response
	body       io.ReadCloser
	encoder    streamOutputEncoder
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

// newTransformingReadCloser prepares a streaming response wrapper that preserves
// the original Content-Encoding on the client-facing response.
func newTransformingReadCloser(resp *http.Response, body io.ReadCloser, contentEncoding string, processor StreamProcessor, onComplete OnCompleteFunc) (*transformingReadCloser, error) {
	readCloser := &transformingReadCloser{resp: resp, body: body, processor: processor, onComplete: onComplete}
	encoder, err := newStreamOutputEncoder(contentEncoding, &readCloser.out)
	if err != nil {
		return nil, err
	}
	readCloser.encoder = encoder
	return readCloser, nil
}

// Read pulls decoded upstream chunks until transformed output is available or
// the stream reaches a terminal state.
//
// StreamProcessor.Stop closes upstream early after its final payload has been
// encoded, which is used by safety filters that refuse a stream mid-flight.
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
				if encodeErr := r.writeEncoded(result.Data); encodeErr != nil {
					r.finished = true
					r.pendingErr = encodeErr
					break
				}
			}
			if result.Stop {
				r.stopped = true
				r.finished = true
				if finishErr := r.encoder.Finish(); finishErr != nil {
					r.pendingErr = finishErr
				}
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
				if encodeErr := r.writeEncoded(finishResult.Data); encodeErr != nil {
					finishErr = errors.Join(finishErr, encodeErr)
				}
			}
			r.finished = true
			if finishResult.Stop {
				r.stopped = true
				if encodeErr := r.encoder.Finish(); encodeErr != nil {
					finishErr = errors.Join(finishErr, encodeErr)
				}
				r.closeBodyAndProcessor()
			} else {
				if encodeErr := r.encoder.Finish(); encodeErr != nil {
					finishErr = errors.Join(finishErr, encodeErr)
				}
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

// Close releases the upstream body, closes the processor, finalizes the encoder,
// and emits completion data unless the processor already stopped the stream.
func (r *transformingReadCloser) Close() error {
	err := r.body.Close()
	r.closeProcessor()
	if encodeErr := r.encoder.Finish(); encodeErr != nil {
		err = errors.Join(err, encodeErr)
	}
	if !r.stopped {
		r.complete()
	}
	return err
}

// writeEncoded writes processor output using the downstream content encoding and
// flushes so SSE clients receive events promptly.
func (r *transformingReadCloser) writeEncoded(data []byte) error {
	if err := r.encoder.Write(data); err != nil {
		return err
	}
	return r.encoder.Flush()
}

// complete invokes onComplete once with the raw decoded upstream stream bytes.
//
// The callback receives upstream bytes before transformation so usage parsers can
// inspect provider-native events even when downstream output was rewritten.
func (r *transformingReadCloser) complete() {
	if r.onComplete == nil {
		return
	}
	r.once.Do(func() {
		r.onComplete(r.resp, append([]byte(nil), r.raw.Bytes()...))
	})
}

// closeBodyAndProcessor closes both upstream body and stream processor after a
// processor-triggered stop.
func (r *transformingReadCloser) closeBodyAndProcessor() {
	_ = r.body.Close()
	r.closeProcessor()
}

// closeProcessor closes the processor at most once.
func (r *transformingReadCloser) closeProcessor() {
	r.closeOnce.Do(func() {
		_ = r.processor.Close()
	})
}

// DecodeResponseBody returns the decoded representation of a complete response
// body according to Content-Encoding.
//
// Supported encodings are identity, gzip, and br. Unsupported encodings return
// an error so callers can avoid auditing or parsing bytes they cannot decode.
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

// newStreamingDecodeReadCloser wraps a streaming response body with an on-read
// decoder for the supported Content-Encoding values.
//
// Ownership transfers to the returned ReadCloser. On construction failure this
// function closes the original body before returning the error.
func newStreamingDecodeReadCloser(body io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return body, nil
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			closeErr := body.Close()
			if closeErr != nil {
				return nil, fmt.Errorf("create gzip stream decoder: %w; close response body: %v", err, closeErr)
			}
			return nil, err
		}
		return joinedReadCloser{reader: reader, closers: []io.Closer{reader, body}}, nil
	case "br":
		return joinedReadCloser{reader: brotli.NewReader(body), closers: []io.Closer{body}}, nil
	default:
		closeErr := body.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("unsupported stream content encoding %q; close response body: %v", contentEncoding, closeErr)
		}
		return nil, fmt.Errorf("unsupported stream content encoding: %s", contentEncoding)
	}
}

// joinedReadCloser couples a decoder reader with every resource that must close
// when the decoded stream is closed.
type joinedReadCloser struct {
	reader  io.Reader
	closers []io.Closer
}

// Read delegates to the decoded reader.
func (r joinedReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

// Close closes all owned resources and joins close errors for the caller.
func (r joinedReadCloser) Close() error {
	var err error
	for _, closer := range r.closers {
		err = errors.Join(err, closer.Close())
	}
	return err
}

// streamOutputEncoder writes transformed stream bytes using the original
// response encoding.
type streamOutputEncoder interface {
	Write([]byte) error
	Flush() error
	Finish() error
}

// newStreamOutputEncoder selects the downstream encoder that matches the
// upstream response Content-Encoding.
func newStreamOutputEncoder(contentEncoding string, out *bytes.Buffer) (streamOutputEncoder, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return &identityStreamEncoder{out: out}, nil
	case "gzip":
		return newGzipStreamEncoder(out), nil
	case "br":
		return newBrotliStreamEncoder(out), nil
	default:
		return nil, fmt.Errorf("unsupported stream content encoding: %s", contentEncoding)
	}
}

// identityStreamEncoder writes unencoded stream bytes directly to the output buffer.
type identityStreamEncoder struct {
	out *bytes.Buffer
}

// Write appends identity-encoded data to the output buffer.
func (e *identityStreamEncoder) Write(data []byte) error {
	_, err := e.out.Write(data)
	return err
}

// Flush is a no-op for identity encoding.
func (e *identityStreamEncoder) Flush() error { return nil }

// Finish is a no-op for identity encoding.
func (e *identityStreamEncoder) Finish() error { return nil }

// gzipStreamEncoder incrementally gzip-encodes transformed SSE output.
type gzipStreamEncoder struct {
	out    *bytes.Buffer
	writer *gzip.Writer
	done   bool
}

// newGzipStreamEncoder creates a gzip encoder that writes into out.
func newGzipStreamEncoder(out *bytes.Buffer) *gzipStreamEncoder {
	encoder := &gzipStreamEncoder{out: out}
	encoder.writer = gzip.NewWriter(out)
	return encoder
}

// Write adds uncompressed data to the gzip stream.
func (e *gzipStreamEncoder) Write(data []byte) error {
	_, err := e.writer.Write(data)
	return err
}

// Flush forces currently encoded gzip bytes into the output buffer unless the
// stream has already finished.
func (e *gzipStreamEncoder) Flush() error {
	if e.done {
		return nil
	}
	return e.writer.Flush()
}

// Finish closes the gzip stream once so downstream clients receive a complete
// compressed payload.
func (e *gzipStreamEncoder) Finish() error {
	if e.done {
		return nil
	}
	e.done = true
	return e.writer.Close()
}

// brotliStreamEncoder incrementally brotli-encodes transformed SSE output.
type brotliStreamEncoder struct {
	out    *bytes.Buffer
	writer *brotli.Writer
	done   bool
}

// newBrotliStreamEncoder creates a brotli encoder that writes into out.
func newBrotliStreamEncoder(out *bytes.Buffer) *brotliStreamEncoder {
	encoder := &brotliStreamEncoder{out: out}
	encoder.writer = brotli.NewWriter(out)
	return encoder
}

// Write adds uncompressed data to the brotli stream.
func (e *brotliStreamEncoder) Write(data []byte) error {
	_, err := e.writer.Write(data)
	return err
}

// Flush forces currently encoded brotli bytes into the output buffer unless the
// stream has already finished.
func (e *brotliStreamEncoder) Flush() error {
	if e.done {
		return nil
	}
	return e.writer.Flush()
}

// Finish closes the brotli stream once so downstream clients receive a complete
// compressed payload.
func (e *brotliStreamEncoder) Finish() error {
	if e.done {
		return nil
	}
	e.done = true
	return e.writer.Close()
}

// isStreamingResponse reports whether the response uses SSE framing.
func isStreamingResponse(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), eventStreamContentType)
}

// shouldCaptureResponseBody decides whether a non-stream response body is safe
// to buffer for completion callbacks.
//
// Binary media and attachments are skipped to avoid excessive memory use and to
// avoid sending undecodable bytes to usage or audit handlers.
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

// restoreResponseBody replaces the consumed response body so httputil can still
// copy the original bytes to the client.
func restoreResponseBody(resp *http.Response, body []byte) {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
}

// completionReadCloser records a non-transformed stream while it is copied to
// the client and invokes onComplete with the decoded body at stream end.
//
// Concurrency: not safe for concurrent reads. Completion is guarded because EOF
// and Close can both be observed by net/http.
type completionReadCloser struct {
	resp       *http.Response
	body       io.ReadCloser
	onComplete OnCompleteFunc
	raw        bytes.Buffer
	once       sync.Once
}

// newCompletionReadCloser wraps a streaming body for completion observation
// without changing client-visible bytes.
func newCompletionReadCloser(resp *http.Response, body io.ReadCloser, onComplete OnCompleteFunc) *completionReadCloser {
	return &completionReadCloser{resp: resp, body: body, onComplete: onComplete}
}

// Read forwards upstream bytes unchanged while keeping a raw copy for the
// completion callback.
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

// Close closes the wrapped body and runs completion if EOF was not reached first.
func (r *completionReadCloser) Close() error {
	err := r.body.Close()
	r.complete()
	return err
}

// complete decodes the captured stream body once and calls onComplete.
func (r *completionReadCloser) complete() {
	r.once.Do(func() {
		decodedBody, err := DecodeResponseBody(r.raw.Bytes(), r.resp.Header.Get("Content-Encoding"))
		if err != nil {
			decodedBody = nil
		}
		r.onComplete(r.resp, decodedBody)
	})
}
