package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout      = 30 * time.Second
	apiVersionHeader    = "Docker-Distribution-API-Version"
	contentDigestHeader = "Docker-Content-Digest"
)

// ClientConfig holds configuration for the registry client.
type ClientConfig struct {
	Address   string
	Username  string
	Password  string
	Insecure  bool
	Timeout   time.Duration
	UserAgent string
}

// Client provides methods to interact with a Docker Registry HTTP API V2.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
	userAgent  string
}

// NewClient creates a new registry API client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("registry address is required")
	}

	baseURL := cfg.Address
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		// Default to HTTP for local registries
		baseURL = "http://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = "localregistry-client/1.0"
	}

	slog.Debug("initialized api client",
		"base_url", baseURL,
		"timeout", timeout,
		"has_credentials", cfg.Username != "",
	)

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
		username:   cfg.Username,
		password:   cfg.Password,
		userAgent:  userAgent,
	}, nil
}

// Ping checks if the registry implements Docker Registry API V2.
func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/v2/", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("ping failed", "error", err)
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("ping response", "status", resp.StatusCode)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		if version := resp.Header.Get(apiVersionHeader); version != "" {
			slog.Debug("registry api version", "version", version)
			return nil
		}
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("registry does not implement V2 API")
	}

	return parseAPIError(resp)
}

// CatalogResponse represents the response from the catalog endpoint.
type CatalogResponse struct {
	Repositories []string `json:"repositories"`
}

// CatalogOptions holds options for listing repositories.
type CatalogOptions struct {
	N    int
	Last string
}

// Catalog retrieves the list of repositories in the registry.
func (c *Client) Catalog(ctx context.Context, opts *CatalogOptions) (*CatalogResponse, string, error) {
	path := "/v2/_catalog"
	if opts != nil {
		params := url.Values{}
		if opts.N > 0 {
			params.Set("n", strconv.Itoa(opts.N))
		}
		if opts.Last != "" {
			params.Set("last", opts.Last)
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("catalog request failed", "error", err)
		return nil, "", fmt.Errorf("catalog request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("catalog response", "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, "", parseAPIError(resp)
	}

	var catalog CatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, "", fmt.Errorf("failed to decode catalog response: %w", err)
	}

	slog.Debug("catalog fetched", "repository_count", len(catalog.Repositories))

	nextLink := parseLinkHeader(resp.Header.Get("Link"))
	return &catalog, nextLink, nil
}

// ListAllRepositories retrieves all repositories using pagination.
func (c *Client) ListAllRepositories(ctx context.Context) ([]string, error) {
	var all []string
	opts := &CatalogOptions{N: 100}

	for {
		catalog, next, err := c.Catalog(ctx, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, catalog.Repositories...)

		if next == "" {
			break
		}
		opts.Last = catalog.Repositories[len(catalog.Repositories)-1]
	}

	return all, nil
}

// TagsResponse represents the response from the tags list endpoint.
type TagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// TagsOptions holds options for listing tags.
type TagsOptions struct {
	N    int
	Last string
}

// ListTags retrieves the list of tags for a repository.
func (c *Client) ListTags(ctx context.Context, name string, opts *TagsOptions) (*TagsResponse, string, error) {
	path := fmt.Sprintf("/v2/%s/tags/list", name)
	if opts != nil {
		params := url.Values{}
		if opts.N > 0 {
			params.Set("n", strconv.Itoa(opts.N))
		}
		if opts.Last != "" {
			params.Set("last", opts.Last)
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("list tags request failed", "repository", name, "error", err)
		return nil, "", fmt.Errorf("list tags request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("list tags response", "repository", name, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, "", parseAPIError(resp)
	}

	var tags TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, "", fmt.Errorf("failed to decode tags response: %w", err)
	}

	slog.Debug("tags fetched", "repository", name, "tag_count", len(tags.Tags))

	nextLink := parseLinkHeader(resp.Header.Get("Link"))
	return &tags, nextLink, nil
}

// ListAllTags retrieves all tags for a repository using pagination.
func (c *Client) ListAllTags(ctx context.Context, name string) ([]string, error) {
	var all []string
	opts := &TagsOptions{N: 100}

	for {
		tags, next, err := c.ListTags(ctx, name, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, tags.Tags...)

		if next == "" || len(tags.Tags) == 0 {
			break
		}
		opts.Last = tags.Tags[len(tags.Tags)-1]
	}

	return all, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	slog.Debug("http request", "method", method, "url", reqURL)

	return req, nil
}

func parseLinkHeader(link string) string {
	if link == "" {
		return ""
	}
	parts := strings.Split(link, ";")
	if len(parts) < 2 {
		return ""
	}
	urlPart := strings.TrimSpace(parts[0])
	urlPart = strings.TrimPrefix(urlPart, "<")
	urlPart = strings.TrimSuffix(urlPart, ">")
	return urlPart
}

// Address returns the registry address (host:port) without protocol prefix.
func (c *Client) Address() string {
	addr := c.baseURL
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	return addr
}
