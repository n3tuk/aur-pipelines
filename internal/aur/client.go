package aur

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type (
	// Client queries the aurweb RPC v5 interface. The zero value is not
	// usable; construct a Client with NewClient.
	Client struct {
		baseURL    string
		httpClient *http.Client
		batchSize  int
	}

	// Option configures a Client.
	Option func(*Client)
)

const (
	// defaultBaseURL is the base URL of the production aurweb RPC interface.
	defaultBaseURL = "https://aur.archlinux.org"
	// defaultTimeout is the default per-request timeout for RPC queries.
	defaultTimeout = 30 * time.Second
	// defaultBatchSize is the maximum number of package names sent in a single
	// info request. The RPC interface accepts many arguments per request, but
	// batching keeps request URLs to a reasonable length.
	defaultBatchSize = 150
	// maxResponseBytes bounds the size of a response body read into memory to
	// guard against unexpectedly large or malicious responses.
	maxResponseBytes = 32 << 20 // 32 MiB
)

// ErrRPC is returned when the RPC interface reports an error response, or the
// response is otherwise not usable.
var ErrRPC = errors.New("aur rpc error")

// WithBaseURL overrides the base URL of the RPC interface. It is primarily
// useful in tests to target a local server.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithBatchSize overrides the maximum number of package names per request. A
// value less than one is ignored.
func WithBatchSize(size int) Option {
	return func(c *Client) {
		if size > 0 {
			c.batchSize = size
		}
	}
}

// NewClient constructs a Client with sensible defaults, applying any options.
func NewClient(opts ...Option) *Client {
	client := &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		batchSize:  defaultBatchSize,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Info queries the RPC info endpoint for the given package names, batching the
// requests as needed, and returns the details of every package that exists in
// the AUR. Names that do not exist in the AUR are simply absent from the
// returned slice. Duplicate names are de-duplicated before querying.
//
// The returned slice preserves the order in which packages are reported by the
// server across batches; callers that need lookup by name should use ByName.
func (c *Client) Info(ctx context.Context, names ...string) ([]Package, error) {
	unique := dedupe(names)
	if len(unique) == 0 {
		return nil, nil
	}

	results := make([]Package, 0, len(unique))

	for start := 0; start < len(unique); start += c.batchSize {
		end := min(start+c.batchSize, len(unique))

		batch, err := c.infoBatch(ctx, unique[start:end])
		if err != nil {
			return nil, err
		}

		results = append(results, batch...)
	}

	return results, nil
}

// infoBatch performs a single info request for the given batch of names.
func (c *Client) infoBatch(ctx context.Context, names []string) ([]Package, error) {
	endpoint, err := c.infoURL(names)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building rpc request: %w", err)
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("performing rpc request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %s", ErrRPC, resp.Status)
	}

	return decodeResponse(resp.Body)
}

// infoURL builds the RPC info endpoint URL for the given package names.
func (c *Client) infoURL(names []string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing base url %q: %w", c.baseURL, err)
	}

	base.Path, err = url.JoinPath(base.Path, "rpc", "v"+strconv.Itoa(rpcVersion), "info")
	if err != nil {
		return "", fmt.Errorf("building rpc path: %w", err)
	}

	query := base.Query()
	for _, name := range names {
		query.Add("arg[]", name)
	}

	base.RawQuery = query.Encode()

	return base.String(), nil
}

// decodeResponse decodes and validates an RPC response body, returning the
// package results or an error for error responses.
func decodeResponse(body io.Reader) ([]Package, error) {
	var envelope response

	decoder := json.NewDecoder(io.LimitReader(body, maxResponseBytes))

	err := decoder.Decode(&envelope)
	if err != nil {
		return nil, fmt.Errorf("decoding rpc response: %w", err)
	}

	if envelope.Type == typeError {
		return nil, fmt.Errorf("%w: %s", ErrRPC, envelope.Error)
	}

	if envelope.Type != typeMultiInfo {
		return nil, fmt.Errorf("%w: unexpected response type %q", ErrRPC, envelope.Type)
	}

	return envelope.Results, nil
}

// dedupe returns the unique, non-empty names from the input, preserving the
// order of first appearance.
func dedupe(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	unique := make([]string, 0, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}

		_, ok := seen[name]
		if ok {
			continue
		}

		seen[name] = struct{}{}
		unique = append(unique, name)
	}

	return unique
}
