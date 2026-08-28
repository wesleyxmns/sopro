package process

import "strings"

type Category string

const (
	CategorySystem      Category = "system"
	CategoryContainer   Category = "container"
	CategoryBrowser     Category = "browser"
	CategoryDatabase    Category = "database"
	CategoryDevelopment Category = "development"
	CategoryJVM         Category = "jvm"
	CategoryOther       Category = "other"
)

type ContextTag string

const (
	ContextDockerCompose ContextTag = "docker-compose"
	ContextGitRepository ContextTag = "git-repository"
	ContextBrowserDebug  ContextTag = "browser-debug"
)

type Classifier struct{}

func DefaultClassifier() Classifier { return Classifier{} }

func (Classifier) Classify(info Info) Category {
	command := strings.ToLower(strings.TrimSpace(info.Command))
	switch command {
	case "systemd", "init", "sshd", "system", "registry", "smss.exe", "csrss.exe", "wininit.exe", "services.exe", "lsass.exe":
		return CategorySystem
	case "docker", "dockerd", "containerd", "containerd-shim", "docker.exe", "com.docker.backend.exe":
		return CategoryContainer
	case "chrome", "chrome.exe", "chromium", "chromium-browser", "firefox", "firefox.exe", "brave", "brave.exe", "msedge.exe":
		return CategoryBrowser
	case "postgres", "postmaster", "mysqld", "mysql", "redis-server", "redis-server.exe", "mongod", "mongod.exe":
		return CategoryDatabase
	case "node", "node.exe", "npm", "pnpm", "yarn", "bun", "deno", "vite":
		return CategoryDevelopment
	case "java", "java.exe", "javaw.exe":
		return CategoryJVM
	default:
		return CategoryOther
	}
}
