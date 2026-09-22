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
	"sync"
	"time"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

var (
	cdpPortPattern = regexp.MustCompile(`--remote-debugging-port=(\d+)`)
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type CDPProvider struct {
	client       HTTPClient
	mu           sync.RWMutex
	detectByPort map[int]cachedDetect
	cacheTTL     time.Duration
}

type cachedDetect struct {
	contexts  []ContextInfo
	fetchedAt time.Time
}

func NewCDPProvider(client ...HTTPClient) *CDPProvider {
	var c HTTPClient = &http.Client{Timeout: 500 * time.Millisecond}
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	}
	return &CDPProvider{client: c, detectByPort: make(map[int]cachedDetect), cacheTTL: 5 * time.Second}
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
	if cached, ok := c.cachedDetect(port); ok {
		return cached
	}
	contexts := c.detectLive(ctx, port)
	c.storeDetect(port, contexts)
	return contexts
}

func (c *CDPProvider) cachedDetect(port int) ([]ContextInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.detectByPort[port]
	if !ok || time.Since(entry.fetchedAt) >= c.cacheTTL {
		return nil, false
	}
	return entry.contexts, true
}

func (c *CDPProvider) storeDetect(port int, contexts []ContextInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.detectByPort[port] = cachedDetect{contexts: contexts, fetchedAt: time.Now()}
}

func (c *CDPProvider) detectLive(ctx context.Context, port int) []ContextInfo {
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

func (c *CDPProvider) CacheTargets(proc processdomain.Info) []CacheTarget {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return nil
	}
	return []CacheTarget{
		{
			Source:   "Navegador",
			ActionID: "cdp.close_blank",
			Label:    "Fechar abas em branco",
			Detail:   fmt.Sprintf("porta %d", port),
			Proc:     proc,
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
	}
}

func (c *CDPProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error) {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return 0, fmt.Errorf("%w: porta CDP não identificada", ErrUnsupported)
	}

	switch actionID {
	case "cdp.close_blank":
		blanks, err := c.listBlankTargets(ctx, port)
		if err != nil {
			return 0, err
		}
		for _, target := range blanks {
			closeReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", port, target.ID), nil)
			if closeReq != nil {
				closeResp, closeErr := c.client.Do(closeReq)
				if closeErr == nil && closeResp != nil {
					closeResp.Body.Close()
				}
			}
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}

// BlankTab is a closable blank tab for confirmation previews.
type BlankTab struct {
	Title string
	URL   string
}

// DisplayName prefers the tab title, falling back to the URL.
func (t BlankTab) DisplayName() string {
	if strings.TrimSpace(t.Title) != "" {
		return t.Title
	}
	return t.URL
}

// BlankTabs lists the tabs close_blank would close for the process.
func (c *CDPProvider) BlankTabs(ctx context.Context, proc processdomain.Info) ([]BlankTab, error) {
	port := extractCDPPort(proc.CommandLine, proc.Command)
	if port <= 0 {
		return nil, fmt.Errorf("%w: porta CDP não identificada", ErrUnsupported)
	}
	targets, err := c.listBlankTargets(ctx, port)
	if err != nil {
		return nil, err
	}
	tabs := make([]BlankTab, 0, len(targets))
	for _, target := range targets {
		tabs = append(tabs, BlankTab{Title: target.Title, URL: target.URL})
	}
	return tabs, nil
}

// listBlankTargets shares the blank-tab definition between execution and
// previews so the confirmation modal lists exactly what will be closed.
func (c *CDPProvider) listBlankTargets(ctx context.Context, port int) ([]cdpTarget, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	var blanks []cdpTarget
	for _, target := range targets {
		if target.Type == "page" && (target.URL == "about:blank" || target.URL == "chrome://newtab/" || target.URL == "edge://newtab/") {
			blanks = append(blanks, target)
		}
	}
	return blanks, nil
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
