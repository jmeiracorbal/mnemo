package agentinit

import (
	"encoding/json"
	"fmt"
	"os"
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
