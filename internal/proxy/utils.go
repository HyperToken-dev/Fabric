package proxy

import (
	"net/url"
	"strings"
)

func parseChannelBaseURL(baseURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, url.InvalidHostError(baseURL)
	}
	if target.Path != "" && target.Path != "/" {
		return nil, url.InvalidHostError("base_url must not include path")
	}
	return target, nil
}
