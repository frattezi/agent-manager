package a2ui

import (
	"encoding/json"
	"fmt"
)

// props are one component's fields, read against the data model its
// bindings point into. Every refusal names the field by its place in the
// document.
type props struct {
	at     string
	fields map[string]json.RawMessage
	data   any
}

const bindingHint = `must be a literal or a {"path": "/pointer"} binding`

func (p props) field(name string) string { return p.at + "." + name }

// binding resolves a {"path": ...} object against the data model. It
// reports false when raw is not a binding.
func (p props) binding(name string, raw json.RawMessage) (any, bool, error) {
	var bound struct {
		Path *string `json:"path"`
	}
	if raw[0] != '{' {
		return nil, false, nil
	}
	if err := json.Unmarshal(raw, &bound); err != nil || bound.Path == nil {
		return nil, false, &Error{p.field(name), bindingHint}
	}
	tokens, err := pointer(p.field(name)+".path", *bound.Path)
	if err != nil {
		return nil, false, err
	}
	return lookup(p.data, tokens), true, nil
}

// text reads a string, literal or bound. A bound value not yet in the model
// reads as empty.
func (p props) text(name string, required bool) (string, error) {
	raw, ok := p.fields[name]
	if !ok || string(raw) == "null" {
		if required {
			return "", &Error{p.field(name), "is required"}
		}
		return "", nil
	}
	value, bound, err := p.binding(name, raw)
	if err != nil {
		return "", err
	}
	if !bound {
		var literal string
		if err := json.Unmarshal(raw, &literal); err != nil {
			return "", &Error{p.field(name), "must be a string; it " + bindingHint}
		}
		return literal, nil
	}
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case json.Number, bool:
		return fmt.Sprint(v), nil
	default:
		return "", &Error{p.field(name), "is bound to a value that is not text"}
	}
}

// flag reads a boolean, literal or bound.
func (p props) flag(name string) (bool, error) {
	raw, ok := p.fields[name]
	if !ok || string(raw) == "null" {
		return false, nil
	}
	value, bound, err := p.binding(name, raw)
	if err != nil {
		return false, err
	}
	if !bound {
		var literal bool
		if err := json.Unmarshal(raw, &literal); err != nil {
			return false, &Error{p.field(name), "must be true or false; it " + bindingHint}
		}
		return literal, nil
	}
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	default:
		return false, &Error{p.field(name), "is bound to a value that is not true or false"}
	}
}

// texts reads a list of strings, literal or bound.
func (p props) texts(name string) ([]string, error) {
	raw, ok := p.fields[name]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	value, bound, err := p.binding(name, raw)
	if err != nil {
		return nil, err
	}
	if !bound {
		var literal []string
		if err := json.Unmarshal(raw, &literal); err != nil {
			return nil, &Error{p.field(name), "must be an array of strings; it " + bindingHint}
		}
		return literal, nil
	}
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, &Error{p.field(name), "is bound to a value that is not an array of strings"}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, &Error{p.field(name), "is bound to an array holding something other than strings"}
		}
		out = append(out, s)
	}
	return out, nil
}

// child reads the one component id a Card or a Button holds.
func (p props) child(name string) ([]childRef, error) {
	id, err := requiredString(p.field(name), p.fields[name])
	if err != nil {
		return nil, err
	}
	return []childRef{{id: id, field: p.field(name)}}, nil
}

// children reads a list of component ids, or a data template, which is
// kept as a gap.
func (p props) children(name string) ([]childRef, error) {
	raw, ok := p.fields[name]
	if !ok {
		return nil, &Error{p.field(name), "is required"}
	}
	var template struct {
		ComponentID *string `json:"componentId"`
	}
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &template); err != nil || template.ComponentID == nil {
			return nil, &Error{p.field(name), `must be an array of component ids or a {"componentId", "path"} template`}
		}
		return []childRef{{template: true}}, nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, &Error{p.field(name), "must be an array of component ids"}
	}
	refs := make([]childRef, 0, len(ids))
	for i, id := range ids {
		field := fmt.Sprintf("%s[%d]", p.field(name), i)
		if id == "" {
			return nil, &Error{field, "must not be empty"}
		}
		refs = append(refs, childRef{id: id, field: field})
	}
	return refs, nil
}

// options reads a ChoicePicker's choices.
func (p props) options(name string) ([]Option, error) {
	raw, ok := p.fields[name]
	if !ok {
		return nil, &Error{p.field(name), "is required"}
	}
	var list []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, &Error{p.field(name), `must be an array of {"label", "value"} objects`}
	}
	if len(list) == 0 {
		return nil, &Error{p.field(name), "must offer at least one choice"}
	}
	out := make([]Option, 0, len(list))
	for i, fields := range list {
		option := props{at: fmt.Sprintf("%s[%d]", p.field(name), i), fields: fields, data: p.data}
		label, err := option.text("label", true)
		if err != nil {
			return nil, err
		}
		value, err := requiredString(option.field("value"), fields["value"])
		if err != nil {
			return nil, err
		}
		out = append(out, Option{Label: label, Value: value})
	}
	return out, nil
}
