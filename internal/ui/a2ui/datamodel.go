package a2ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// readDataModel applies one updateDataModel message: value is written at
// path, the whole model when path is absent or "/", and a message without a
// value removes what is at path.
func (b *builder) readDataModel(at string, body map[string]json.RawMessage) error {
	path := "/"
	if raw, ok := body["path"]; ok {
		if err := json.Unmarshal(raw, &path); err != nil {
			return &Error{at + ".path", "must be a JSON Pointer string such as \"/round/q1\""}
		}
	}
	tokens, err := pointer(at+".path", path)
	if err != nil {
		return err
	}
	raw, present := body["value"]
	var value any
	if present {
		if value, err = decodeValue(raw); err != nil {
			return &Error{at + ".value", "must be JSON: " + err.Error()}
		}
	}
	updated, err := write(b.data, tokens, value, present)
	if err != nil {
		return &Error{at + ".path", err.Error()}
	}
	b.data = updated
	return nil
}

// pointer splits a JSON Pointer into its unescaped tokens.
func pointer(field, path string) ([]string, error) {
	if path == "" || path == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, &Error{field, fmt.Sprintf("%q must start with \"/\"", path)}
	}
	tokens := strings.Split(path[1:], "/")
	for i, token := range tokens {
		tokens[i] = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
	}
	return tokens, nil
}

func write(into any, tokens []string, value any, present bool) (any, error) {
	if len(tokens) == 0 {
		if !present {
			return nil, nil
		}
		return value, nil
	}
	head, rest := tokens[0], tokens[1:]
	switch node := into.(type) {
	case []any:
		index, err := strconv.Atoi(head)
		if err != nil || index < 0 || index >= len(node) {
			return nil, fmt.Errorf("%q is not an index of the array there", head)
		}
		child, err := write(node[index], rest, value, present)
		if err != nil {
			return nil, err
		}
		node[index] = child
		return node, nil
	case map[string]any:
		if len(rest) == 0 && !present {
			delete(node, head)
			return node, nil
		}
		child, err := write(node[head], rest, value, present)
		if err != nil {
			return nil, err
		}
		node[head] = child
		return node, nil
	case nil:
		if !present {
			return nil, nil
		}
		child, err := write(nil, rest, value, present)
		if err != nil {
			return nil, err
		}
		return map[string]any{head: child}, nil
	default:
		return nil, fmt.Errorf("%q is under a value that is neither an object nor an array", head)
	}
}

// lookup reads what is at path, or nil when nothing is there yet.
func lookup(data any, tokens []string) any {
	for _, token := range tokens {
		switch node := data.(type) {
		case map[string]any:
			data = node[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return nil
			}
			data = node[index]
		default:
			return nil
		}
	}
	return data
}
