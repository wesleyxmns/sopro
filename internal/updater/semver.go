package updater

import (
	"strconv"
	"strings"
)

// IsNewer retorna true se a versão remota 'latest' for estritamente mais recente que a versão 'current'.
func IsNewer(latest, current string) bool {
	latest = cleanVersion(latest)
	current = cleanVersion(current)

	if current == "dev" || current == "none" || current == "unknown" || current == "" {
		// Em ambiente de desenvolvimento local, qualquer release oficial válida é tratada como mais recente
		return latest != "" && latest != "dev"
	}

	if latest == "" || latest == "dev" {
		return false
	}

	latParts := parseSemVerParts(latest)
	curParts := parseSemVerParts(current)

	for i := 0; i < 3; i++ {
		if latParts[i] > curParts[i] {
			return true
		}
		if latParts[i] < curParts[i] {
			return false
		}
	}

	return false
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	return v
}

func parseSemVerParts(v string) [3]int {
	var parts [3]int
	tokens := strings.Split(v, ".")
	for i := 0; i < len(tokens) && i < 3; i++ {
		if val, err := strconv.Atoi(tokens[i]); err == nil {
			parts[i] = val
		}
	}
	return parts
}
