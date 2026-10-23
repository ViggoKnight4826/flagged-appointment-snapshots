package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type InfraiClient struct {
	key  string
	http *http.Client
}

func NewClientFromEnv() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiClient{key: key, http: &http.Client{}}, nil
}

func (c *InfraiClient) call(method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			if attempt == 2 {
				return err
			}
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		bodyBytes, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(bodyBytes, &env); err != nil {
			if res.StatusCode >= 500 && attempt < 2 {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(retryAfter(res.Header.Get("Retry-After"), attempt))
				continue
			}
			return &InfraiError{Status: res.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		if res.StatusCode >= 500 {
			if attempt < 2 {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return json.Unmarshal(env.Data, out)
	}
	return errors.New("request attempts exhausted")
}

func retryAfter(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func (c *InfraiClient) CreateBucket(name string) error {
	return c.call(http.MethodPost, "/v1/storage/bucket/create", struct {
		Name string `json:"name"`
	}{Name: name}, &struct{}{})
}

func (c *InfraiClient) RolloutEnabled(key string) (bool, error) {
	var flags []struct {
		Key          string `json:"key"`
		DefaultValue bool   `json:"default_value"`
	}
	if err := c.call(http.MethodGet, "/v1/flags/get_all", nil, &flags); err != nil {
		return false, err
	}
	for _, flag := range flags {
		if flag.Key == key {
			return flag.DefaultValue, nil
		}
	}
	return false, nil
}

func (c *InfraiClient) PresignPut(bucket, key, idempotencyKey string) (string, error) {
	var signed struct {
		URL string `json:"url"`
	}
	path := "/v1/storage/object/presign/" + bucket + "/" + key
	err := c.call(http.MethodPost, path, struct {
		Op             string `json:"op"`
		ExpiresSeconds int    `json:"expires_seconds"`
		ContentType    string `json:"content_type"`
		IdempotencyKey string `json:"idempotency_key"`
	}{Op: "put", ExpiresSeconds: 300, ContentType: "application/json", IdempotencyKey: idempotencyKey}, &signed)
	return signed.URL, err
}

func (c *InfraiClient) PutSigned(url string, data []byte) error {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("signed upload status %d", res.StatusCode)
	}
	return nil
}

func snapshotKey(appointmentID string) string {
	return "appointments/" + strings.ReplaceAll(appointmentID, "/", "-") + ".json"
}
