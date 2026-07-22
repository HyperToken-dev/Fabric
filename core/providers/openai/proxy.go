package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"

	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
)

func New(opts coreproxy.Options) *coreproxy.Proxy {
	if opts.Rewrite == nil {
		opts.Rewrite = defaultRewrite
	}

	return coreproxy.New(opts)
}

func defaultRewrite(pr *httputil.ProxyRequest, upstream coreproxy.Upstream) error {
	if pr.Out.URL.Path != "/v1/chat/completions" {
		return nil
	}
	return injectChatStreamOptions(pr.Out)
}

func injectChatStreamOptions(req *http.Request) error {
	if req.Body == nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	closeErr := req.Body.Close()
	if err != nil {
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	restoreRequestBody(req, body)
	if closeErr != nil {
		return closeErr
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	stream, ok := payload["stream"].(bool)
	if !ok || !stream {
		return nil
	}

	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		streamOptions = make(map[string]any)
	}
	streamOptions["include_usage"] = true
	payload["stream_options"] = streamOptions

	newBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	restoreRequestBody(req, newBody)
	return nil
}

func restoreRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}
