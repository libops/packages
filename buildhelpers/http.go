package buildhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPStatusError struct {
	URL        string
	StatusCode int
}

func (e *HTTPStatusError) Error() string { return fmt.Sprintf("%s: HTTP %d", e.URL, e.StatusCode) }

// FetchMetadata bounds both time and memory, even with an injected HTTP client.
func FetchMetadata(ctx context.Context, client *http.Client, endpoint string, headers http.Header) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{URL: endpoint, StatusCode: response.StatusCode}
	}
	const limit = 8 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("metadata exceeds download limit")
	}
	return data, nil
}

// JSONResponse shares bounded API response handling. GET/DELETE 404 is returned
// to callers for explicit missing-resource/idempotency decisions; writes fail.
func JSONResponse(client *http.Client, req *http.Request, result any) (int, error) {
	ctx, cancel := context.WithTimeout(req.Context(), time.Minute)
	defer cancel()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status == http.StatusNotFound && (req.Method == "GET" || req.Method == "DELETE") {
		return status, nil
	}
	if status < 200 || status >= 300 {
		return status, &HTTPStatusError{URL: req.URL.Scheme + "://" + req.URL.Host + req.URL.Path, StatusCode: status}
	}
	if result != nil {
		decoder := json.NewDecoder(io.LimitReader(response.Body, 32<<20))
		if err := decoder.Decode(result); err != nil {
			return status, err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return status, fmt.Errorf("API response contains trailing or oversized data")
		}
	}
	return status, nil
}
