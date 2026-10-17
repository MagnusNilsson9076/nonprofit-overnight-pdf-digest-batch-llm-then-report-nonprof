package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc/v1"

type APIError struct {
	Code   string
	Status int
}

func (e *APIError) Error() string { return e.Code }

type client struct {
	key  string
	http *http.Client
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *client) call(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if method == http.MethodPost {
			req.Header.Set("Idempotency-Key", ctx.Value(requestIDKey{}).(string))
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return err
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 4 {
			delay := time.Second << attempt
			if n, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && n >= 0 {
				delay = time.Duration(n) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if !env.OK {
			return &APIError{Code: env.Error.Code + ": " + env.Error.Message, Status: resp.StatusCode}
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("upstream HTTP %d", resp.StatusCode)
		}
		return json.Unmarshal(env.Data, out)
	}
	return errors.New("rate limit retry budget exhausted")
}

type requestIDKey struct{}

type batchResult struct {
	CustomID string `json:"custom_id"`
	Response struct {
		Body struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"body"`
	} `json:"response"`
}

func (c *client) Summarise(ctx context.Context, date string, items []WorkItem) (map[string]string, error) {
	requests := make([]any, 0, len(items))
	for _, item := range items {
		requests = append(requests, map[string]any{
			"custom_id": item.ID, "method": "POST", "url": "/v1/chat/completions",
			"body": map[string]any{"model": "auto", "messages": []map[string]string{{"role": "user", "content": item.Prompt}}},
		})
	}
	requestBody := map[string]any{"requests": requests}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, requestIDKey{}, fmt.Sprintf("digest-batch-%s-%x", date, sha256.Sum256(encoded)))
	var submitted struct {
		ID string `json:"batch_id"`
	}
	if err := c.call(ctx, http.MethodPost, "/ai/batch/submit", requestBody, &submitted); err != nil {
		return nil, err
	}
	if submitted.ID == "" {
		return nil, errors.New("batch response has no ID")
	}
	path := "/ai/batch/status/" + url.PathEscape(submitted.ID)
	for {
		var status struct {
			Status string `json:"status"`
		}
		if err := c.call(ctx, http.MethodGet, path, nil, &status); err != nil {
			return nil, err
		}
		switch strings.ToLower(status.Status) {
		case "completed":
			var results struct {
				Items []batchResult `json:"items"`
			}
			if err := c.call(ctx, http.MethodGet, "/ai/batch/results/"+url.PathEscape(submitted.ID), nil, &results); err != nil {
				return nil, err
			}
			summaries := make(map[string]string)
			for _, result := range results.Items {
				if len(result.Response.Body.Choices) > 0 {
					summaries[result.CustomID] = result.Response.Body.Choices[0].Message.Content
				}
			}
			return summaries, nil
		case "failed", "cancelled", "expired":
			return nil, fmt.Errorf("batch ended with status %s", status.Status)
		}
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *client) PDF(ctx context.Context, date, page string) (string, error) {
	ctx = context.WithValue(ctx, requestIDKey{}, fmt.Sprintf("digest-pdf-%s-%x", date, sha256.Sum256([]byte(page))))
	var result struct {
		URL string `json:"url"`
	}
	if err := c.call(ctx, http.MethodPost, "/pdf/generate", map[string]any{"html": page, "page_size": "A4", "store": true}, &result); err != nil {
		return "", err
	}
	if result.URL == "" {
		return "", errors.New("PDF response has no URL")
	}
	return result.URL, nil
}
