package infrai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRequestCodeRetriesWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/sms/otp" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Idempotency-Key") != "send-17" {
			t.Fatal("missing authorization or idempotency header")
		}
		if requests == 1 {
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"0"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":{"message":"retry later"}}`))}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{},"metadata":{}}`))}, nil
	})}
	client, err := NewSMSCodes("test-key")
	if err != nil {
		t.Fatal(err)
	}
	client.baseURL = "https://example.test"
	client.http = httpClient
	client.sleep = func(context.Context, time.Duration) error { return nil }
	if err := client.RequestCode(context.Background(), "+14155550123", "send-17"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}
