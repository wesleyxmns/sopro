package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	processdomain "sopro/internal/process"
)

type mockHTTPClient struct {
	responses map[string]string
	errors    map[string]error
	requests  []*http.Request
}

func newMockHTTPClient() *mockHTTPClient {
	return &mockHTTPClient{
		responses: make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	url := req.URL.String()
	if err, ok := m.errors[url]; ok {
		return nil, err
	}
	if body, ok := m.responses[url]; ok {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}
	return nil, errors.New("request failed: unmocked url " + url)
}

func TestCDPProviderSupports(t *testing.T) {
	p := NewCDPProvider(newMockHTTPClient())

	if !p.Supports(processdomain.Info{Category: processdomain.CategoryBrowser}) {
		t.Fatal("expected support for CategoryBrowser")
	}
	if !p.Supports(processdomain.Info{CommandLine: "google-chrome --remote-debugging-port=9222"}) {
		t.Fatal("expected support for --remote-debugging-port")
	}
	if p.Supports(processdomain.Info{Category: processdomain.CategoryDatabase, Command: "postgres"}) {
		t.Fatal("did not expect support for postgres")
	}
}

func TestCDPProviderDetect(t *testing.T) {
	client := newMockHTTPClient()
	jsonList := `[
		{"id": "tab-1", "title": "Dashboard", "type": "page", "url": "https://example.com"},
		{"id": "tab-2", "title": "Blank", "type": "page", "url": "about:blank"},
		{"id": "worker-1", "title": "ServiceWorker", "type": "service_worker", "url": "https://example.com/sw.js"}
	]`
	client.responses["http://127.0.0.1:9222/json/list"] = jsonList

	p := NewCDPProvider(client)
	proc := processdomain.Info{
		Category:    processdomain.CategoryBrowser,
		CommandLine: "/opt/google/chrome/chrome --remote-debugging-port=9222 --user-data-dir=/tmp/c",
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	if contexts[0].Tag != processdomain.ContextBrowserDebug {
		t.Fatalf("tag = %q; want %q", contexts[0].Tag, processdomain.ContextBrowserDebug)
	}
	if contexts[0].Label != "CDP :9222 (2 abas)" {
		t.Fatalf("label = %q; want 'CDP :9222 (2 abas)'", contexts[0].Label)
	}
	if contexts[0].Details["port"] != "9222" || contexts[0].Details["tabs"] != "2" {
		t.Fatalf("details = %+v", contexts[0].Details)
	}
}

func TestCDPProviderActionsAndExecute(t *testing.T) {
	client := newMockHTTPClient()
	jsonList := `[
		{"id": "tab-1", "title": "Docs", "type": "page", "url": "https://golang.org"},
		{"id": "tab-2", "title": "Blank", "type": "page", "url": "about:blank"}
	]`
	client.responses["http://127.0.0.1:9222/json/list"] = jsonList
	client.responses["http://127.0.0.1:9222/json/close/tab-2"] = "Target is closing"

	p := NewCDPProvider(client)
	proc := processdomain.Info{
		CommandLine: "chrome --remote-debugging-port=9222",
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 2 || actions[0].ID != "cdp.close_blank" || actions[1].ID != "cdp.discard_inactive" {
		t.Fatalf("actions = %+v", actions)
	}

	ctx := context.Background()
	if err := p.Execute(ctx, "cdp.close_blank", proc); err != nil {
		t.Fatalf("failed to execute cdp.close_blank: %v", err)
	}
	if err := p.Execute(ctx, "cdp.discard_inactive", proc); err != nil {
		t.Fatalf("failed to execute cdp.discard_inactive: %v", err)
	}
	if len(client.requests) < 2 {
		t.Fatalf("expected at least 2 HTTP requests, got %d", len(client.requests))
	}
}
