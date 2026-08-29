// Package httpclient is the tiny JSON-over-HTTP transport shared by the
// food-brain's backend clients (mealie, skolmaten, retailer, llm). It owns the
// request/do/decode mechanics so each client is only its own URL shapes,
// headers, and payloads; the backend error prefix keeps failures attributable
// while the transport itself stays backend-agnostic.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal JSON-over-HTTP client. Every error it produces is
// prefixed with the backend name (e.g. "mealie") so failures stay attributable.
type Client struct {
	baseURL string
	prefix  string
	http    *http.Client
}

// StatusError is a non-2xx HTTP response from a backend. It carries the status
// code (and the backend's {"error": "..."} detail, when present) so callers can
// detect specific conditions — e.g. a stale 401 credential — by inspecting the
// status rather than parsing the error message. Its Error() text is identical
// to the plain error the client produced before this type existed, so existing
// message-based assertions keep holding.
type StatusError struct {
	Backend    string
	StatusCode int
	Path       string
	Detail     string
}

func (e *StatusError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: HTTP %d for %s: %s", e.Backend, e.StatusCode, e.Path, e.Detail)
	}
	return fmt.Sprintf("%s: HTTP %d for %s", e.Backend, e.StatusCode, e.Path)
}

// New returns a Client for baseURL (trailing "/" trimmed) whose errors are
// prefixed with prefix ("mealie", "skolmaten", "adapter", "olla", ...).
func New(baseURL, prefix string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		prefix:  prefix,
		http:    &http.Client{Timeout: timeout},
	}
}

// GetJSON GETs path and decodes a 2xx response body into out. headers, when
// non-nil, is called with the request so a backend can attach auth headers.
func (c *Client) GetJSON(ctx context.Context, path string, out any, headers func(*http.Request)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if headers != nil {
		headers(req)
	}
	return c.do(req, out)
}

// PostJSON marshals body and POSTs it to path, decoding a 2xx response body
// into out. headers, when non-nil, is called with the request so a backend can
// attach auth headers.
func (c *Client) PostJSON(ctx context.Context, path string, body, out any, headers func(*http.Request)) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%s: marshal %s payload: %w", c.prefix, path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if headers != nil {
		headers(req)
	}
	return c.do(req, out)
}

// PatchJSON marshals body and PATCHes it to path, decoding a 2xx response body
// into out. headers, when non-nil, is called with the request so a backend can
// attach auth headers.
func (c *Client) PatchJSON(ctx context.Context, path string, body, out any, headers func(*http.Request)) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%s: marshal %s payload: %w", c.prefix, path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if headers != nil {
		headers(req)
	}
	return c.do(req, out)
}

// do performs the request: transport errors are wrapped with the backend
// prefix, non-2xx is an error (carrying the backend's {"error": "..."} body
// when present), and a 2xx body is decoded into out.
func (c *Client) do(req *http.Request, out any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", c.prefix, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&apiErr)
		return &StatusError{
			Backend:    c.prefix,
			StatusCode: res.StatusCode,
			Path:       req.URL.Path,
			Detail:     apiErr.Error,
		}
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: decode %s: %w", c.prefix, req.URL.Path, err)
	}
	return nil
}
