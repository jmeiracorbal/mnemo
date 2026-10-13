package agentinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func removeMCPServer(path, rootKey, serverName string) (bool, error) {
	return updateJSONFile(path, func(root map[string]any) bool {
		servers, ok := root[rootKey].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := servers[serverName]; !ok {
			return false
		}
		delete(servers, serverName)
		if len(servers) == 0 {
			delete(root, rootKey)
		}
		return true
	})
}

func removeNestedMCPServer(path, rootKey, nestedKey, serverName string) (bool, error) {
	return updateJSONFile(path, func(root map[string]any) bool {
		parent, ok := root[rootKey].(map[string]any)
		if !ok {
			return false
		}
		servers, ok := parent[nestedKey].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := servers[serverName]; !ok {
			return false
		}
		delete(servers, serverName)
		if len(servers) == 0 {
			delete(parent, nestedKey)
		}
		if len(parent) == 0 {
			delete(root, rootKey)
		}
		return true
	})
}

func removeHookCommands(path string, eventCommands map[string]string) (bool, error) {
	return updateJSONFile(path, func(root map[string]any) bool {
		hooks, ok := root["hooks"].(map[string]any)
		if !ok {
			return false
		}
		changed := false
		for event, command := range eventCommands {
			items, ok := hooks[event].([]any)
			if !ok {
				continue
			}
			kept := make([]any, 0, len(items))
			for _, item := range items {
				if containsCommand(item, command) {
					changed = true
					continue
				}
				kept = append(kept, item)
			}
			if len(kept) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = kept
			}
		}
		if len(hooks) == 0 {
			delete(root, "hooks")
		}
		return changed
	})
}

func updateJSONFile(path string, mutate func(map[string]any) bool) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var root map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	if !mutate(root) {
		return false, nil
	}
	if len(root) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0644); err != nil {
		return false, err
	}
	return true, nil
}

func containsCommand(v any, command string) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "command" {
				if got, ok := item.(string); ok && got == command {
					return true
				}
			}
			if containsCommand(item, command) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if containsCommand(item, command) {
				return true
			}
		}
	}
	return false
}

// upsertOwnedHooks replaces every hook whose command lives under ownedDir with
// the entries in hooks (event name to matcher groups), keeping the user's other
// hooks and settings. It writes path only when the result differs, and returns
// whether it did. A nil hooks map removes the owned hooks.
func upsertOwnedHooks(path, ownedDir string, hooks map[string][]any) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	root := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	before, err := json.Marshal(root)
	if err != nil {
		return false, err
	}

	existing, _ := root["hooks"].(map[string]any)
	merged := map[string]any{}
	for event, groups := range existing {
		list, ok := groups.([]any)
		if !ok {
			merged[event] = groups
			continue
		}
		if kept := withoutOwnedHooks(list, ownedDir); len(kept) > 0 {
			merged[event] = kept
		}
	}
	for event, groups := range hooks {
		list, _ := merged[event].([]any)
		merged[event] = append(list, groups...)
	}
	if len(merged) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"] = merged
	}

	after, err := json.Marshal(root)
	if err != nil {
		return false, err
	}
	if string(before) == string(after) {
		return false, nil
	}
	if len(data) == 0 && len(root) == 0 {
		return false, nil
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	return true, WriteFile(path, append(out, '\n'))
}

// withoutOwnedHooks drops hooks whose command is under ownedDir from matcher
// groups, and groups left without hooks.
func withoutOwnedHooks(groups []any, ownedDir string) []any {
	prefix := filepath.Clean(ownedDir) + string(filepath.Separator)
	var kept []any
	for _, group := range groups {
		g, ok := group.(map[string]any)
		if !ok {
			kept = append(kept, group)
			continue
		}
		hooks, ok := g["hooks"].([]any)
		if !ok {
			kept = append(kept, group)
			continue
		}
		var own []any
		for _, hook := range hooks {
			h, ok := hook.(map[string]any)
			if command, _ := h["command"].(string); ok && strings.HasPrefix(command, prefix) {
				continue
			}
			own = append(own, hook)
		}
		if len(own) == 0 {
			continue
		}
		copied := make(map[string]any, len(g))
		for k, v := range g {
			copied[k] = v
		}
		copied["hooks"] = own
		kept = append(kept, copied)
	}
	return kept
}
