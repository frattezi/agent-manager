package a2ui

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// rootID is the component a surface's tree starts from.
const rootID = "root"

// maxNodes bounds the tree a document expands to. Components are an
// adjacency list, so a few that each name the next twice describe a tree
// that doubles per level.
const maxNodes = 10000

// Document is a parsed surface: its id and the tree under its root, with
// every binding already resolved against the data model.
type Document struct {
	SurfaceID string
	Root      Node
}

// Node is one component of the tree. Type is the A2UI component name; a
// type outside the subset is a gap, with Gap set and Reason saying why.
type Node struct {
	ID        string
	Type      string
	Text      string
	Variant   string
	Label     string
	Value     string
	Direction string
	Checked   bool
	Multiple  bool
	Options   []Option
	Children  []Node
	Gap       bool
	Reason    string
}

// Option is one choice of a ChoicePicker and whether it is chosen.
type Option struct {
	Label    string
	Value    string
	Selected bool
}

// Error refuses a document, naming the field at fault so the agent that
// wrote it can fix that field and publish again.
type Error struct {
	Field   string
	Problem string
}

func (e *Error) Error() string { return e.Field + ": " + e.Problem }

// VersionError refuses a document written for another version of A2UI.
type VersionError struct {
	Field    string
	Found    string
	Expected string
}

func (e *VersionError) Error() string {
	if e.Found == "" {
		return fmt.Sprintf("%s is missing; this build reads A2UI %q", e.Field, e.Expected)
	}
	return fmt.Sprintf("%s is %q; this build reads A2UI %q", e.Field, e.Found, e.Expected)
}

type envelope struct {
	Version          *json.RawMessage `json:"version"`
	CreateSurface    json.RawMessage  `json:"createSurface"`
	UpdateComponents json.RawMessage  `json:"updateComponents"`
	UpdateDataModel  json.RawMessage  `json:"updateDataModel"`
}

type rawComponent struct {
	at    string
	props map[string]json.RawMessage
}

// childRef is a child a component names, and the field that named it.
type childRef struct {
	id       string
	field    string
	template bool
}

type builder struct {
	components map[string]rawComponent
	data       any
	visiting   map[string]bool
	count      int
}

// Parse reads a document. It refuses one whose messages claim another
// version, or whose fields cannot describe a surface, and names the field.
func Parse(data []byte) (Document, error) {
	var messages []json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return Document{}, &Error{"document", "must be a JSON array of A2UI messages: " + err.Error()}
	}
	if len(messages) == 0 {
		return Document{}, &Error{"document", "holds no messages"}
	}
	b := builder{components: map[string]rawComponent{}, visiting: map[string]bool{}}
	surface, updated := "", false
	for i, message := range messages {
		at := fmt.Sprintf("messages[%d]", i)
		var env envelope
		if err := json.Unmarshal(message, &env); err != nil {
			return Document{}, &Error{at, "must be an object: " + err.Error()}
		}
		if err := checkVersion(at+".version", env.Version); err != nil {
			return Document{}, err
		}
		for _, part := range []struct {
			name string
			body json.RawMessage
			read func(string, map[string]json.RawMessage) error
		}{
			{"createSurface", env.CreateSurface, nil},
			{"updateComponents", env.UpdateComponents, b.readComponents},
			{"updateDataModel", env.UpdateDataModel, b.readDataModel},
		} {
			if len(part.body) == 0 {
				continue
			}
			field := at + "." + part.name
			var body map[string]json.RawMessage
			if err := json.Unmarshal(part.body, &body); err != nil || body == nil {
				return Document{}, &Error{field, "must be an object"}
			}
			id, err := requiredString(field+".surfaceId", body["surfaceId"])
			if err != nil {
				return Document{}, err
			}
			if surface == "" {
				surface = id
			} else if id != surface {
				return Document{}, &Error{field + ".surfaceId", fmt.Sprintf("is %q, but this document describes surface %q; publish one surface per document", id, surface)}
			}
			if part.read == nil {
				continue
			}
			if err := part.read(field, body); err != nil {
				return Document{}, err
			}
			if part.name == "updateComponents" {
				updated = true
			}
		}
	}
	if !updated {
		return Document{}, &Error{"document", "has no updateComponents message, so there is nothing to draw"}
	}
	if _, ok := b.components[rootID]; !ok {
		return Document{}, &Error{"document", fmt.Sprintf("no component has id %q; the tree starts there", rootID)}
	}
	root, err := b.build(rootID)
	if err != nil {
		return Document{}, err
	}
	return Document{SurfaceID: surface, Root: root}, nil
}

func checkVersion(field string, raw *json.RawMessage) error {
	if raw == nil {
		return &VersionError{Field: field, Expected: Version}
	}
	var found string
	if err := json.Unmarshal(*raw, &found); err != nil {
		return &VersionError{Field: field, Found: string(*raw), Expected: Version}
	}
	if found != Version {
		return &VersionError{Field: field, Found: found, Expected: Version}
	}
	return nil
}

func (b *builder) readComponents(at string, body map[string]json.RawMessage) error {
	field := at + ".components"
	var list []json.RawMessage
	if err := json.Unmarshal(body["components"], &list); err != nil || list == nil {
		return &Error{field, "must be an array of components"}
	}
	for i, raw := range list {
		where := fmt.Sprintf("%s[%d]", field, i)
		var props map[string]json.RawMessage
		if err := json.Unmarshal(raw, &props); err != nil || props == nil {
			return &Error{where, "must be an object"}
		}
		id, err := requiredString(where+".id", props["id"])
		if err != nil {
			return err
		}
		if _, err := requiredString(where+".component", props["component"]); err != nil {
			return err
		}
		b.components[id] = rawComponent{at: where, props: props}
	}
	return nil
}

func (b *builder) build(id string) (Node, error) {
	raw := b.components[id]
	b.count++
	if b.count > maxNodes {
		return Node{}, &Error{raw.at, fmt.Sprintf("the tree expands past %d components; name each child once", maxNodes)}
	}
	kind, _ := requiredString("", raw.props["component"])
	known, ok := subset[kind]
	if !ok {
		return Node{ID: id, Type: kind, Gap: true, Reason: "not in the subset this build draws"}, nil
	}
	node, refs, err := known.read(props{at: raw.at, fields: raw.props, data: b.data})
	if err != nil {
		return Node{}, err
	}
	node.ID, node.Type = id, kind
	b.visiting[id] = true
	defer delete(b.visiting, id)
	for _, ref := range refs {
		if ref.template {
			node.Children = append(node.Children, Node{Type: "template", Gap: true, Reason: "children from a data template are not drawn by this build"})
			continue
		}
		if _, ok := b.components[ref.id]; !ok {
			return Node{}, &Error{ref.field, fmt.Sprintf("no component has id %q", ref.id)}
		}
		if b.visiting[ref.id] {
			return Node{}, &Error{ref.field, fmt.Sprintf("%q contains itself, so the tree never ends", ref.id)}
		}
		child, err := b.build(ref.id)
		if err != nil {
			return Node{}, err
		}
		node.Children = append(node.Children, child)
	}
	return node, nil
}

func requiredString(field string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", &Error{field, "is required"}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", &Error{field, "must be a string"}
	}
	if s == "" {
		return "", &Error{field, "must not be empty"}
	}
	return s, nil
}

func decodeValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	err := decoder.Decode(&value)
	return value, err
}
