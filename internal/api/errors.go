// Package api provides a client for interacting with Docker Registry HTTP API V2.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Registry API error codes as defined in the specification.
const (
	ErrCodeManifestUnknown = "MANIFEST_UNKNOWN"
	ErrCodeNameUnknown     = "NAME_UNKNOWN"
)

// APIError represents a single error returned by the registry API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// Error implements the error interface.
func (e APIError) Error() string {
	if e.Detail != nil {
		return fmt.Sprintf("%s: %s (detail: %v)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// APIErrors represents multiple errors returned by the registry API.
type APIErrors struct {
	Errors     []APIError `json:"errors"`
	StatusCode int        `json:"-"`
}

// Error implements the error interface.
func (e APIErrors) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("registry error: status %d", e.StatusCode)
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e.Errors[0].Error(), len(e.Errors)-1)
}

// parseAPIError parses an error response from the registry API.
func parseAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response: %w", err)
	}

	var apiErrs APIErrors
	if err := json.Unmarshal(body, &apiErrs); err != nil {
		return fmt.Errorf("registry error: status %d, body: %s", resp.StatusCode, string(body))
	}
	apiErrs.StatusCode = resp.StatusCode
	return apiErrs
}

// IsNotFound returns true if the error indicates a 404 Not Found response.
func IsNotFound(err error) bool {
	if apiErrs, ok := err.(APIErrors); ok {
		return apiErrs.StatusCode == http.StatusNotFound
	}
	return false
}
