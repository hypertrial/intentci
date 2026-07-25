package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	jsonMarshal     = json.Marshal
	httpNewRequest  = http.NewRequestWithContext
	ioReadAll       = io.ReadAll
)

// HTTPProvider POSTs JSON to a configured endpoint.
type HTTPProvider struct {
	URL     string
	Timeout time.Duration
	Token   string
	Client  *http.Client
}

func (p *HTTPProvider) Analyze(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(p.URL) == "" {
		return Response{}, fmt.Errorf("http semantic provider url is empty")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	payload, err := jsonMarshal(req)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := httpNewRequest(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.Token)
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("http semantic provider request failed: %w", err)
	}
	defer res.Body.Close()
	body, err := ioReadAll(io.LimitReader(res.Body, MaxInputBytes+1024))
	if err != nil {
		return Response{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = res.Status
		}
		return Response{}, fmt.Errorf("http semantic provider returned %s: %s", res.Status, msg)
	}
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return Response{}, fmt.Errorf("http semantic provider returned invalid JSON: %w", err)
	}
	if resp.Findings == nil {
		resp.Findings = []Finding{}
	}
	return resp, nil
}
