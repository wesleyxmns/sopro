package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultGitHubRepo = "wesleyxmns/sopro"
	cacheTTL          = 24 * time.Hour
	httpTimeout       = 4 * time.Second
)

// Checker gerencia a verificação de versões e cache local.
type Checker struct {
	Repo       string
	HTTPClient *http.Client
	CachePath  string
}

// NewChecker cria uma nova instância de Checker.
func NewChecker() *Checker {
	return &Checker{
		Repo: defaultGitHubRepo,
		HTTPClient: &http.Client{
			Timeout: httpTimeout,
		},
		CachePath: DefaultCachePath(),
	}
}

// DefaultCachePath retorna o caminho padrão para o arquivo de cache de atualização.
func DefaultCachePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "sopro", "update_cache.json")
}

// Check verifica se há uma atualização disponível.
func (c *Checker) Check(ctx context.Context, currentVersion string, force bool) (*ReleaseInfo, bool, error) {
	if !force {
		if cached, ok := c.readCache(); ok {
			if cached.LatestRelease != nil {
				return cached.LatestRelease, IsNewer(cached.LatestRelease.Version, currentVersion), nil
			}
		}
	}

	release, err := c.fetchLatestRelease(ctx)
	if err != nil {
		return nil, false, err
	}

	c.writeCache(release)
	isNew := IsNewer(release.Version, currentVersion)
	return release, isNew, nil
}

// Cached retorna a release armazenada localmente enquanto o cache ainda estiver válido.
// Ele não realiza acesso à rede e permite que interfaces exibam o estado conhecido
// imediatamente enquanto uma revalidação acontece em segundo plano.
func (c *Checker) Cached(currentVersion string) (*ReleaseInfo, bool, time.Time, bool) {
	cached, ok := c.readCache()
	if !ok || cached.LatestRelease == nil {
		return nil, false, time.Time{}, false
	}
	return cached.LatestRelease,
		IsNewer(cached.LatestRelease.Version, currentVersion),
		cached.LastCheckedAt,
		true
}

type githubReleasePayload struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *Checker) fetchLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sopro-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("servidor respondeu com status HTTP %d", resp.StatusCode)
	}

	var payload githubReleasePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta do GitHub: %w", err)
	}

	version := strings.TrimPrefix(payload.TagName, "v")
	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH

	var assetURL, assetName, checksumURL string
	for _, asset := range payload.Assets {
		if asset.Name == "checksums.txt" {
			checksumURL = asset.BrowserDownloadURL
			continue
		}
		if strings.Contains(asset.Name, targetOS) && strings.Contains(asset.Name, targetArch) {
			assetURL = asset.BrowserDownloadURL
			assetName = asset.Name
		}
	}

	return &ReleaseInfo{
		Version:      version,
		TagName:      payload.TagName,
		PublishedAt:  payload.PublishedAt,
		HTMLURL:      payload.HTMLURL,
		ReleaseNotes: payload.Body,
		AssetURL:     assetURL,
		AssetName:    assetName,
		ChecksumURL:  checksumURL,
	}, nil
}

func (c *Checker) readCache() (*CacheState, bool) {
	if c.CachePath == "" {
		return nil, false
	}
	data, err := os.ReadFile(c.CachePath)
	if err != nil {
		return nil, false
	}
	var state CacheState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false
	}
	if time.Since(state.LastCheckedAt) > cacheTTL {
		return nil, false
	}
	return &state, true
}

func (c *Checker) writeCache(release *ReleaseInfo) {
	if c.CachePath == "" || release == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(c.CachePath), 0755)
	state := CacheState{
		LastCheckedAt: time.Now(),
		LatestRelease: release,
	}
	data, err := json.Marshal(state)
	if err == nil {
		_ = os.WriteFile(c.CachePath, data, 0644)
	}
}
