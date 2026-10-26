package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type RestTransport struct {
	basePath   string
	httpClient *http.Client
}

func NewTransport(basePath string, opts ...Option) *RestTransport {
	rt := &RestTransport{
		basePath:   basePath,
		httpClient: &http.Client{},
	}

	for _, opt := range opts {
		opt(rt)
	}

	return rt
}

func (rt *RestTransport) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("could not marshal request body: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, rt.basePath+path, body)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := rt.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(resp.Body)
		return &HTTPError{
			err:        fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, string(respBody)),
			StatusCode: resp.StatusCode,
			Response:   resp,
		}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("could not decode response: %w", err)
		}
	}

	return nil
}

func (rt *RestTransport) GetJSON(ctx context.Context, path string, out any) error {
	return rt.doJSON(ctx, http.MethodGet, path, nil, out)
}

func (rt *RestTransport) PostJSON(ctx context.Context, path string, body, out any) error {
	return rt.doJSON(ctx, http.MethodPost, path, body, out)
}

func (rt *RestTransport) PutJSON(ctx context.Context, path string, body, out any) error {
	return rt.doJSON(ctx, http.MethodPut, path, body, out)
}

func (rt *RestTransport) PatchJSON(ctx context.Context, path string, body, out any) error {
	return rt.doJSON(ctx, http.MethodPatch, path, body, out)
}

func (rt *RestTransport) Delete(ctx context.Context, path string) error {
	return rt.doJSON(ctx, http.MethodDelete, path, nil, nil)
}
