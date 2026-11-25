package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Only finish_turn's top-level public field is exposed, never content/reasoning
// or arguments belonging to rolls, state changes, or other tools.
func partialPublicText(args, field string) string {
	d := json.NewDecoder(strings.NewReader(args))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return ""
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return ""
		}
		start := int(d.InputOffset())
		if key == field {
			rest := strings.TrimSpace(args[start:])
			if !strings.HasPrefix(rest, ":") {
				return ""
			}
			rest = strings.TrimSpace(rest[1:])
			if !strings.HasPrefix(rest, `"`) {
				return ""
			}
			var text string
			dec := json.NewDecoder(strings.NewReader(rest))
			if dec.Decode(&text) == nil {
				return text
			}
			// Close only a syntactically valid prefix; incomplete escapes (including
			// UTF-16 surrogate pairs) remain buffered until the next provider delta.
			for n := len(rest); n > 0; n-- {
				if json.Unmarshal([]byte(rest[:n]+`"`), &text) == nil && !strings.ContainsRune(text, '\uFFFD') {
					return text
				}
			}
			return ""
		}
		var skip json.RawMessage
		if d.Decode(&skip) != nil {
			return ""
		}
	}
	return ""
}

func readGameModelStream(r io.Reader, field string, publish func(string) error) (gameModelMessage, error) {
	result := gameModelMessage{Role: "assistant"}
	scanner := bufio.NewScanner(io.LimitReader(r, (1<<20)+1))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	total, done := 0, false
	var data []string
	consume := func() error {
		raw := strings.Join(data, "\n")
		data = nil
		if raw == "" {
			return nil
		}
		if raw == "[DONE]" {
			done = true
			return nil
		}
		var event struct {
			Error   json.RawMessage `json:"error"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(raw), &event) != nil || len(event.Error) > 0 {
			return fmt.Errorf("invalid model stream")
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 {
				return fmt.Errorf("multiple model choices")
			}
			for _, delta := range choice.Delta.ToolCalls {
				if delta.Index != 0 {
					return fmt.Errorf("parallel model calls unsupported")
				}
				if len(result.ToolCalls) == 0 {
					result.ToolCalls = append(result.ToolCalls, gameToolCall{Type: "function"})
				}
				call := &result.ToolCalls[0]
				call.ID += delta.ID
				call.Function.Name += delta.Function.Name
				call.Function.Arguments += delta.Function.Arguments
				if call.Function.Name == "finish_turn" && publish != nil {
					if text := partialPublicText(call.Function.Arguments, field); text != "" {
						if err := publish(text); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > 1<<20 {
			return result, fmt.Errorf("model stream too large")
		}
		if line == "" {
			if err := consume(); err != nil {
				return result, err
			}
			if done {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if !done && len(data) > 0 {
		if err := consume(); err != nil {
			return result, err
		}
	}
	if scanner.Err() != nil || !done {
		return result, fmt.Errorf("model stream interrupted")
	}
	return result, nil
}
