package buildhelpers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJSONResponseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		method, body string
		status       int
		ok           bool
	}{{"GET", `{"name":"release"}`, 200, true}, {"GET", `{"name":"release"}{}`, 200, false}, {"GET", "invalid", 200, false}, {"GET", "", 404, true}, {"DELETE", "", 404, true}, {"POST", "", 404, false}, {"GET", "", 500, false}} {
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			if _, ok := r.Context().Deadline(); !ok {
				t.Fatal("missing request deadline")
			}
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		request, _ := http.NewRequestWithContext(context.Background(), tc.method, "https://api.example.invalid/releases", nil)
		var data map[string]any
		status, err := JSONResponse(client, request, &data)
		if (err == nil) != tc.ok || status != tc.status {
			t.Fatalf("%+v: %d %v", tc, status, err)
		}
	}
}
func TestFetchMetadataStatusAndCancellation(t *testing.T) {
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("missing"))}, nil
	})}
	_, err := FetchMetadata(context.Background(), client, "https://example.invalid/missing", nil)
	var status *HTTPStatusError
	if !errors.As(err, &status) || status.StatusCode != 404 {
		t.Fatalf("untyped metadata error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	if _, err := FetchMetadata(ctx, client, "https://example.invalid/cancel", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
