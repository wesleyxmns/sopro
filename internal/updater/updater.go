package updater

import (
	"time"
)

// ReleaseInfo contém os metadados de uma release oficial no GitHub.
type ReleaseInfo struct {
	Version      string    `json:"version"`
	TagName      string    `json:"tag_name"`
	PublishedAt  time.Time `json:"published_at"`
	HTMLURL      string    `json:"html_url"`
	ReleaseNotes string    `json:"release_notes"`
	AssetURL     string    `json:"asset_url"`
	AssetName    string    `json:"asset_name"`
	ChecksumURL  string    `json:"checksum_url"`
}

// CacheState armazena o estado da última checagem de atualização local.
type CacheState struct {
	LastCheckedAt time.Time    `json:"last_checked_at"`
	LatestRelease *ReleaseInfo `json:"latest_release"`
}
