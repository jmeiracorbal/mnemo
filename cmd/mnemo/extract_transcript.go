package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// runExtractTranscript reads a JSONL transcript file and prints the text
// content of all assistant messages. Intended for passive capture in hooks.
//
// Usage: mnemo extract-transcript /path/to/transcript.jsonl
func runExtractTranscript() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mnemo extract-transcript <file.jsonl>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		// Silent failure: transcript may not exist yet.
		return
	}
	defer f.Close()

	type block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var msg message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil || msg.Role != "assistant" {
			continue
		}

		// Try plain string content first.
		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil {
			if text != "" {
				lines = append(lines, text)
			}
			continue
		}

		// Try array of content blocks.
		var blocks []block
		if err := json.Unmarshal(msg.Content, &blocks); err == nil {
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					lines = append(lines, b.Text)
				}
			}
		}
	}

	fmt.Println(strings.Join(lines, "\n"))
}
