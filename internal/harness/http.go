package harness

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

func PostJSON(url string, headers map[string]string, body string, timeoutSeconds int) (string, error) {
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &HTTPError{Status: resp.StatusCode, Body: string(data)}
	}
	return string(data), nil
}
