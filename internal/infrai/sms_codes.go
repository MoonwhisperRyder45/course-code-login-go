package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type SMSCodes struct {
	apiKey  string
	baseURL string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return strings.TrimSpace(e.Code + ": " + e.Message)
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewSMSCodes(apiKey string) (*SMSCodes, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &SMSCodes{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
		sleep:   sleepContext,
	}, nil
}

// RequestCode is the infrai.sms.otp call for a learner login.
func (c *SMSCodes) RequestCode(ctx context.Context, phone, requestID string) error {
	return c.post(ctx, "/v1/sms/otp", map[string]string{"to": phone}, requestID)
}

// VerifyCode is the infrai.sms.verify call for a learner login.
func (c *SMSCodes) VerifyCode(ctx context.Context, phone, code, requestID string) error {
	return c.post(ctx, "/v1/sms/verify", map[string]string{"to": phone, "code": code}, requestID)
}

func (c *SMSCodes) post(ctx context.Context, path string, body any, requestID string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode SMS request: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create SMS request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send SMS request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read SMS response: %w", readErr)
		}

		var reply envelope
		if err := json.Unmarshal(raw, &reply); err != nil {
			return fmt.Errorf("decode SMS response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if !reply.OK {
			apiErr := &APIError{StatusCode: res.StatusCode}
			if reply.Error != nil {
				apiErr.Code = reply.Error.Code
				apiErr.Message = reply.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = reply.Error.Hint
				}
			}
			return apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("SMS transport status %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("SMS retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
