package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	// Version é a versão semântica do Sopro (ex: v0.1.0). Injetado em tempo de compilação via ldflags.
	Version = "dev"
	// Commit é o hash Git do commit de compilação. Injetado via ldflags.
	Commit = "none"
	// BuildDate é a data UTC de compilação (ex: 2026-08-31T12:00:00Z). Injetado via ldflags.
	BuildDate = "unknown"
	// BuiltBy identifica a ferramenta que realizou o build (ex: goreleaser, make). Injetado via ldflags.
	BuiltBy = "source"
)

// Info retorna a string formatada de versão para exibição no terminal.
func Info() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("sopro %s", Version))
	if Commit != "" && Commit != "none" {
		sb.WriteString(fmt.Sprintf(" (%s)", Commit))
	}
	sb.WriteString(fmt.Sprintf("\n  compilado em: %s", BuildDate))
	sb.WriteString(fmt.Sprintf("\n  plataforma:   %s/%s", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("\n  runtime:      %s", runtime.Version()))
	if BuiltBy != "" && BuiltBy != "source" {
		sb.WriteString(fmt.Sprintf("\n  criado por:   %s", BuiltBy))
	}
	return sb.String()
}

// Short retorna uma representação compacta da versão (ex: "sopro v0.1.0").
func Short() string {
	return fmt.Sprintf("sopro %s", Version)
}
