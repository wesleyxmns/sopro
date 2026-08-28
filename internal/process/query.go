package process

import (
	"sort"
	"strings"
	"unicode"
)

type SortField string

const (
	SortDefault SortField = ""
	SortMemory  SortField = "memory"
	SortCPU     SortField = "cpu"
	SortCommand SortField = "command"
)

type Query struct {
	Search   string
	Category Category
	Sort     SortField
}

func (query Query) Apply(processes []Info) []Info {
	result := make([]Info, 0, len(processes))
	for _, candidate := range processes {
		if query.Category != "" && candidate.Category != query.Category {
			continue
		}
		if query.Search != "" && !FuzzyMatch(query.Search, candidate.Command+" "+candidate.User) {
			continue
		}
		result = append(result, candidate)
	}
	if query.Sort != "" {
		sort.SliceStable(result, func(left, right int) bool {
			switch query.Sort {
			case SortCPU:
				return result[left].CPUPct > result[right].CPUPct
			case SortCommand:
				return strings.ToLower(result[left].Command) < strings.ToLower(result[right].Command)
			case SortMemory:
				return result[left].MemoryBytes > result[right].MemoryBytes
			default:
				return false
			}
		})
	}
	return result
}

func FuzzyMatch(pattern, value string) bool {
	patternRunes := []rune(strings.ToLower(strings.TrimSpace(pattern)))
	if len(patternRunes) == 0 {
		return true
	}
	position := 0
	for _, candidate := range []rune(strings.ToLower(value)) {
		if unicode.IsSpace(patternRunes[position]) && unicode.IsSpace(candidate) || patternRunes[position] == candidate {
			position++
			if position == len(patternRunes) {
				return true
			}
		}
	}
	return false
}

func GroupByCategory(processes []Info) map[Category][]Info {
	groups := make(map[Category][]Info)
	for _, candidate := range processes {
		groups[candidate.Category] = append(groups[candidate.Category], candidate)
	}
	return groups
}
