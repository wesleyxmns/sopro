package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	processdomain "sopro/internal/process"
)

var (
	cdpPortPattern = regexp.MustCompile(`--remote-debugging-port=(\d+)`)
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type CDPProvider struct {
	client HTTPClient
}

func NewCDPProvider(client ...HTTPClient) *CDPProvider {
	var c HTTPClient = &http.Client{Timeout: 500 * time.Millisecond}
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	}
	return &CDPProvider{client: c}
}

func (c *CDPProvider) Name() string {
	return "cdp"
}

func (c *CDPProvider) Supports(proc processdomain.Info) bool {
	if proc.Category == processdomain.CategoryBrowser {
		return true
	}
	cmd := strings.ToLower(proc.Command + " " + proc.CommandLine)
	return strings.Contains(cmd, "chrome") || strings.Contains(cmd, "chromium") ||
		strings.Contains(cmd, "brave") || strings.Contains(cmd, "edge") || strings.Contains(cmd, "remote-debugging-port")
}

type cdpTarget struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

func (c *CDPProvider) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return nil
	}

	details := map[string]string{
		"port": strconv.Itoa(port),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
	if err == nil {
		resp, err := c.client.Do(req)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var targets []cdpTarget
			if err := json.Unmarshal(body, &targets); err == nil {
				pages := 0
				for _, t := range targets {
					if t.Type == "page" {
						pages++
					}
				}
				details["tabs"] = strconv.Itoa(pages)
				return []ContextInfo{
					{
						Tag:     processdomain.ContextBrowserDebug,
						Label:   fmt.Sprintf("CDP :%d (%d abas)", port, pages),
						Details: details,
					},
				}
			}
		}
	}

	return []ContextInfo{
		{
			Tag:     processdomain.ContextBrowserDebug,
			Label:   fmt.Sprintf("CDP :%d", port),
			Details: details,
		},
	}
}

func (c *CDPProvider) Actions(ctx context.Context, proc processdomain.Info) []Action {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return nil
	}
	return []Action{
		{
			ID:          "cdp.close_blank",
			Scope:       ScopeBrowser,
			Label:       "fechar abas em branco",
			Description: fmt.Sprintf("Fecha páginas sobre:blank abertas no navegador (porta %d)", port),
			Danger:      false,
		},
		{
			ID:          "cdp.discard_inactive",
			Scope:       ScopeBrowser,
			Label:       "suspender abas inativas",
			Description: fmt.Sprintf("Descarta da memória abas inativas do navegador (porta %d)", port),
			Danger:      false,
		},
	}
}

func (c *CDPProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) error {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return fmt.Errorf("%w: porta CDP não identificada", ErrUnsupported)
	}

	switch actionID {
	case "cdp.close_blank":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
		if err != nil {
			return err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var targets []cdpTarget
		if err := json.Unmarshal(body, &targets); err != nil {
			return err
		}
		for _, target := range targets {
			if target.Type == "page" && (target.URL == "about:blank" || target.URL == "chrome://newtab/" || target.URL == "edge://newtab/") {
				closeReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", port, target.ID), nil)
				if closeReq != nil {
					closeResp, closeErr := c.client.Do(closeReq)
					if closeErr == nil && closeResp != nil {
						closeResp.Body.Close()
					}
				}
			}
		}
		return nil
	case "cdp.discard_inactive":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
		if err != nil {
			return err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var targets []cdpTarget
		if err := json.Unmarshal(body, &targets); err != nil {
			return err
		}
		for _, target := range targets {
			if target.Type == "page" && (target.URL == "about:blank" || target.URL == "chrome://newtab/") {
				closeReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", port, target.ID), nil)
				if closeReq != nil {
					closeResp, closeErr := c.client.Do(closeReq)
					if closeErr == nil && closeResp != nil {
						closeResp.Body.Close()
					}
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}

func extractCDPPort(commandLine, command string) int {
	text := commandLine + " " + command
	if match := cdpPortPattern.FindStringSubmatch(text); len(match) > 1 {
		if port, err := strconv.Atoi(match[1]); err == nil && port > 0 {
			return port
		}
	}
	return 0
}
