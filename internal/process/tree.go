package process

import "sort"

type TreeEntry struct {
	Info
	Depth int
}

func BuildTree(processes []Info) []TreeEntry {
	byParent := make(map[int32][]Info)
	known := make(map[int32]bool, len(processes))
	for _, candidate := range processes {
		known[candidate.PID] = true
	}
	for _, candidate := range processes {
		parent := candidate.ParentPID
		if parent == candidate.PID || !known[parent] {
			parent = 0
		}
		byParent[parent] = append(byParent[parent], candidate)
	}
	for parent := range byParent {
		sort.SliceStable(byParent[parent], func(left, right int) bool {
			return byParent[parent][left].MemoryBytes > byParent[parent][right].MemoryBytes
		})
	}
	result := make([]TreeEntry, 0, len(processes))
	visited := make(map[int32]bool, len(processes))
	var visit func(Info, int)
	visit = func(candidate Info, depth int) {
		if visited[candidate.PID] {
			return
		}
		visited[candidate.PID] = true
		result = append(result, TreeEntry{Info: candidate, Depth: depth})
		for _, child := range byParent[candidate.PID] {
			visit(child, depth+1)
		}
	}
	for _, root := range byParent[0] {
		visit(root, 0)
	}
	for _, candidate := range processes {
		visit(candidate, 0)
	}
	return result
}
