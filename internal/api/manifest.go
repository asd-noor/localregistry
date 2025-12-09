package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Common manifest media types.
const (
	MediaTypeManifestV2   = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeOCIManifest  = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIIndex     = "application/vnd.oci.image.index.v1+json"
	MediaTypeDockerConfig = "application/vnd.docker.container.image.v1+json"
)

// Manifest represents a retrieved manifest with its metadata.
type Manifest struct {
	ContentType string
	Digest      string
	Body        []byte
}

// GetManifest retrieves a manifest by name and reference (tag or digest).
func (c *Client) GetManifest(ctx context.Context, name, reference string) (*Manifest, error) {
	path := fmt.Sprintf("/v2/%s/manifests/%s", name, reference)

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", fmt.Sprintf("%s, %s, %s, %s",
		MediaTypeManifestV2,
		MediaTypeManifestList,
		MediaTypeOCIManifest,
		MediaTypeOCIIndex,
	))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get manifest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest body: %w", err)
	}

	return &Manifest{
		ContentType: resp.Header.Get("Content-Type"),
		Digest:      resp.Header.Get(contentDigestHeader),
		Body:        body,
	}, nil
}

// HeadManifest checks if a manifest exists and returns its metadata without the body.
func (c *Client) HeadManifest(ctx context.Context, name, reference string) (*Manifest, error) {
	path := fmt.Sprintf("/v2/%s/manifests/%s", name, reference)

	req, err := c.newRequest(ctx, http.MethodHead, path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", fmt.Sprintf("%s, %s, %s, %s",
		MediaTypeManifestV2,
		MediaTypeManifestList,
		MediaTypeOCIManifest,
		MediaTypeOCIIndex,
	))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("head manifest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	return &Manifest{
		ContentType: resp.Header.Get("Content-Type"),
		Digest:      resp.Header.Get(contentDigestHeader),
	}, nil
}

// ManifestExists checks if a manifest exists for the given name and reference.
func (c *Client) ManifestExists(ctx context.Context, name, reference string) (bool, error) {
	_, err := c.HeadManifest(ctx, name, reference)
	if err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteManifest deletes a manifest by name and digest.
// Note: Manifests can only be deleted by digest, not by tag.
func (c *Client) DeleteManifest(ctx context.Context, name, digest string) error {
	path := fmt.Sprintf("/v2/%s/manifests/%s", name, digest)

	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete manifest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return parseAPIError(resp)
	}

	return nil
}

// DeleteManifestByTag retrieves the digest for a tag and deletes the manifest.
func (c *Client) DeleteManifestByTag(ctx context.Context, name, tag string) error {
	manifest, err := c.HeadManifest(ctx, name, tag)
	if err != nil {
		return fmt.Errorf("failed to get manifest digest for tag %s: %w", tag, err)
	}

	if manifest.Digest == "" {
		return fmt.Errorf("no digest returned for tag %s", tag)
	}

	return c.DeleteManifest(ctx, name, manifest.Digest)
}
