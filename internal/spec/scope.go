package spec

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// CheckWriteScope verifies that every path in changedPaths falls within the
// declared write_paths of taskID in def. It fails closed on unknown tasks,
// invalid write_paths, unsafe changed paths, or any path outside write_paths.
func CheckWriteScope(def Definition, taskID string, changedPaths []string) ([]string, error) {
	var task *Task
	for i := range def.Tasks {
		if def.Tasks[i].ID == taskID {
			task = &def.Tasks[i]
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("unknown task %q", taskID)
	}
	if len(task.WritePaths) == 0 {
		return nil, fmt.Errorf("task %s has no declared write_paths", taskID)
	}
	for _, allowed := range task.WritePaths {
		if !ValidPath(allowed) {
			return nil, fmt.Errorf("task %s has unsafe write path %q", taskID, allowed)
		}
		for _, r := range allowed {
			if r > unicode.MaxASCII {
				return nil, fmt.Errorf("task %s write path %q must use ASCII", taskID, allowed)
			}
		}
	}

	seenViolations := make(map[string]bool)
	var violations []string
	for _, changed := range changedPaths {
		if !ValidPath(changed) {
			return nil, fmt.Errorf("unsafe changed path %q", changed)
		}
		if pathWithinWriteScopes(changed, task.WritePaths) {
			continue
		}
		if !seenViolations[changed] {
			seenViolations[changed] = true
			violations = append(violations, changed)
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return violations, fmt.Errorf("task %s modified paths outside write_paths: %s", taskID, strings.Join(violations, ", "))
	}
	return nil, nil
}

func pathWithinWriteScopes(path string, writePaths []string) bool {
	for _, allowed := range writePaths {
		if path == allowed || strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}
	return false
}
