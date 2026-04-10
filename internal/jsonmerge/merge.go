// Package jsonmerge provides a deep-merge operation for JSON files.
// It is used by the mnemo json-merge command to idempotently patch
// configuration files (mcp.json, hooks.json) without external tools.
package jsonmerge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MergeFile reads filePath (or starts from {} if it does not exist),
// reads a JSON patch from stdin, deep-merges the patch into the file,
// and writes the result back atomically.
//
// Merge rules:
//   - Objects are merged recursively: patch keys are added or overwrite target.
//   - Arrays are concatenated; items already present (by JSON equality) are not duplicated.
//   - Scalars: patch value wins.
//
// Returns true if the file was changed, false if it was already up to date.
func MergeFile(filePath string) (changed bool, err error) {
	// If filePath is a symlink, resolve it so we write to the real file and
	// do not replace the symlink with a regular file (e.g. dotfiles setups).
	if fi, statErr := os.Lstat(filePath); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, evalErr := filepath.EvalSymlinks(filePath)
		if evalErr != nil {
			return false, fmt.Errorf("resolve symlink %s: %w", filePath, evalErr)
		}
		filePath = resolved
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat %s: %w", filePath, statErr)
	}

	// Read existing content.
	var target any = map[string]any{}
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", filePath, err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &target); err != nil {
			return false, fmt.Errorf("parse %s: %w", filePath, err)
		}
	}

	// Read patch from stdin.
	var patch any
	if err := json.NewDecoder(os.Stdin).Decode(&patch); err != nil {
		return false, fmt.Errorf("read patch from stdin: %w", err)
	}

	merged := deepMerge(target, patch)

	mergedBytes, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal result: %w", err)
	}
	mergedBytes = append(mergedBytes, '\n')

	// Detect no-op: normalize existing content through a round-trip and compare.
	if len(data) > 0 {
		var norm any
		if json.Unmarshal(data, &norm) == nil {
			normBytes, _ := json.MarshalIndent(norm, "", "  ")
			normBytes = append(normBytes, '\n')
			if string(mergedBytes) == string(normBytes) {
				return false, nil
			}
		}
	}

	// Write atomically: temp file in the same directory, then rename.
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return false, fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".mnemo-jsonmerge-*")
	if err != nil {
		return false, fmt.Errorf("tempfile: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			if removeErr := os.Remove(tmpName); removeErr != nil && !os.IsNotExist(removeErr) {
				// best-effort cleanup; primary error takes precedence
				_ = removeErr
			}
		}
	}()
	if _, err = tmp.Write(mergedBytes); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write temp: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp: %w", err)
	}
	if err = os.Rename(tmpName, filePath); err != nil {
		return false, fmt.Errorf("rename: %w", err)
	}
	return true, nil
}

// deepMerge merges patch into target following the rules described in MergeFile.
func deepMerge(target, patch any) any {
	tMap, tIsMap := target.(map[string]any)
	pMap, pIsMap := patch.(map[string]any)
	if tIsMap && pIsMap {
		result := make(map[string]any, len(tMap))
		for k, v := range tMap {
			result[k] = v
		}
		for k, pv := range pMap {
			if tv, exists := result[k]; exists {
				result[k] = deepMerge(tv, pv)
			} else {
				result[k] = pv
			}
		}
		return result
	}

	tSlice, tIsSlice := target.([]any)
	pSlice, pIsSlice := patch.([]any)
	if tIsSlice && pIsSlice {
		return dedupConcat(tSlice, pSlice)
	}

	// Scalar or type mismatch: patch wins.
	return patch
}

// dedupConcat returns a concatenated slice where items from b that are
// already present in a (by JSON equality) are skipped.
func dedupConcat(a, b []any) []any {
	result := make([]any, len(a))
	copy(result, a)
	for _, item := range b {
		if !containsJSON(result, item) {
			result = append(result, item)
		}
	}
	return result
}

// containsJSON reports whether slice contains an item that is JSON-equal to item.
func containsJSON(slice []any, item any) bool {
	itemBytes, err := json.Marshal(item)
	if err != nil {
		return false
	}
	for _, existing := range slice {
		existingBytes, err := json.Marshal(existing)
		if err != nil {
			continue
		}
		if string(existingBytes) == string(itemBytes) {
			return true
		}
	}
	return false
}
