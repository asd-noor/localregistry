package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// BlobInfo contains metadata about a blob.
type BlobInfo struct {
	Digest        string
	ContentLength int64
}

// HeadBlob checks if a blob exists and returns its metadata.
func (c *Client) HeadBlob(ctx context.Context, name, digest string) (*BlobInfo, error) {
	path := fmt.Sprintf("/v2/%s/blobs/%s", name, digest)

	req, err := c.newRequest(ctx, http.MethodHead, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("head blob request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	return &BlobInfo{
		Digest:        resp.Header.Get(contentDigestHeader),
		ContentLength: resp.ContentLength,
	}, nil
}

// BlobExists checks if a blob exists in the registry.
func (c *Client) BlobExists(ctx context.Context, name, digest string) (bool, error) {
	_, err := c.HeadBlob(ctx, name, digest)
	if err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetBlob retrieves a blob's content by name and digest.
func (c *Client) GetBlob(ctx context.Context, name, digest string) (io.ReadCloser, *BlobInfo, error) {
	path := fmt.Sprintf("/v2/%s/blobs/%s", name, digest)

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("get blob request failed", "repository", name, "digest", digest, "error", err)
		return nil, nil, fmt.Errorf("get blob request failed: %w", err)
	}

	if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusFound {
		location := resp.Header.Get("Location")
		resp.Body.Close()
		if location == "" {
			return nil, nil, fmt.Errorf("redirect response missing Location header")
		}
		slog.Debug("following blob redirect", "location", location)
		redirectReq, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create redirect request: %w", err)
		}
		resp, err = c.httpClient.Do(redirectReq)
		if err != nil {
			return nil, nil, fmt.Errorf("redirect request failed: %w", err)
		}
	}

	slog.Debug("get blob response", "repository", name, "digest", digest, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, nil, parseAPIError(resp)
	}

	info := &BlobInfo{
		Digest:        resp.Header.Get(contentDigestHeader),
		ContentLength: resp.ContentLength,
	}

	slog.Debug("blob stream opened", "repository", name, "digest", digest, "size", info.ContentLength)

	return resp.Body, info, nil
}

// DeleteBlob deletes a blob by name and digest.
func (c *Client) DeleteBlob(ctx context.Context, name, digest string) error {
	path := fmt.Sprintf("/v2/%s/blobs/%s", name, digest)

	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("delete blob request failed", "repository", name, "digest", digest, "error", err)
		return fmt.Errorf("delete blob request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("delete blob response", "repository", name, "digest", digest, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusAccepted {
		return parseAPIError(resp)
	}

	slog.Info("blob deleted", "repository", name, "digest", digest)
	return nil
}
