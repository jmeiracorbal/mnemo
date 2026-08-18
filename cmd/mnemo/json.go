package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/jmeiracorbal/mnemo/internal/jsonmerge"
)

// runJSON reads JSON from stdin and extracts a value by key path.
// Intended for use in hook scripts as a replacement for inline python3 -c.
//
// Usage: mnemo json KEY [KEY ...]
// Examples:
//
//	mnemo json session_id
//	mnemo json workspace_roots 0
//	mnemo json tool_info transcript_path
func runJSON() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo json KEY [KEY ...]")
		os.Exit(1)
	}

	var data any
	if err := json.NewDecoder(os.Stdin).Decode(&data); err != nil {
		fmt.Println("")
		return
	}

	value := data
	for _, key := range os.Args[2:] {
		switch v := value.(type) {
		case map[string]any:
			value = v[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(v) {
				value = nil
			} else {
				value = v[i]
			}
		default:
			value = nil
		}
		if value == nil {
			break
		}
	}

	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case float64:
		if v == float64(int64(v)) {
			fmt.Println(int64(v))
		} else {
			fmt.Println(v)
		}
	case bool:
		fmt.Println(v)
	default:
		fmt.Println("")
	}
}

// runJSONMerge reads a JSON patch from stdin and deep-merges it into FILE.
// Usage: mnemo json-merge <file>
func runJSONMerge() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo json-merge <file>")
		os.Exit(1)
	}
	changed, err := jsonmerge.MergeFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mnemo: json-merge: %v\n", err)
		os.Exit(1)
	}
	if changed {
		fmt.Println("updated")
	} else {
		fmt.Println("already up to date")
	}
}
