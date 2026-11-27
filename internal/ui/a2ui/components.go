package a2ui

import "fmt"

func readText(p props) (Node, []childRef, error) {
	text, err := p.text("text", true)
	if err != nil {
		return Node{}, nil, err
	}
	variant, err := p.text("variant", false)
	return Node{Text: text, Variant: variant}, nil, err
}

func readCard(p props) (Node, []childRef, error) {
	refs, err := p.child("child")
	return Node{}, refs, err
}

func readColumn(p props) (Node, []childRef, error) {
	refs, err := p.children("children")
	return Node{}, refs, err
}

func readRow(p props) (Node, []childRef, error) {
	refs, err := p.children("children")
	return Node{}, refs, err
}

func readList(p props) (Node, []childRef, error) {
	refs, err := p.children("children")
	if err != nil {
		return Node{}, nil, err
	}
	direction, err := p.text("direction", false)
	return Node{Direction: direction}, refs, err
}

func readDivider(p props) (Node, []childRef, error) {
	return Node{}, nil, nil
}

func readButton(p props) (Node, []childRef, error) {
	refs, err := p.child("child")
	if err != nil {
		return Node{}, nil, err
	}
	variant, err := p.text("variant", false)
	return Node{Variant: variant}, refs, err
}

func readTextField(p props) (Node, []childRef, error) {
	label, err := p.text("label", true)
	if err != nil {
		return Node{}, nil, err
	}
	value, err := p.text("value", false)
	if err != nil {
		return Node{}, nil, err
	}
	variant, err := p.text("variant", false)
	return Node{Label: label, Value: value, Variant: variant}, nil, err
}

func readCheckBox(p props) (Node, []childRef, error) {
	label, err := p.text("label", true)
	if err != nil {
		return Node{}, nil, err
	}
	checked, err := p.flag("value")
	return Node{Label: label, Checked: checked}, nil, err
}

func readChoicePicker(p props) (Node, []childRef, error) {
	label, err := p.text("label", false)
	if err != nil {
		return Node{}, nil, err
	}
	variant, err := p.text("variant", false)
	if err != nil {
		return Node{}, nil, err
	}
	options, err := p.options("options")
	if err != nil {
		return Node{}, nil, err
	}
	chosen, err := p.texts("value")
	if err != nil {
		return Node{}, nil, err
	}
	multiple := variant == "multipleSelection"
	if !multiple && len(chosen) > 1 {
		return Node{}, nil, &Error{p.field("value"), `chooses more than one option; set "variant": "multipleSelection" or choose one`}
	}
	for n, value := range chosen {
		found := false
		for i := range options {
			if options[i].Value == value {
				options[i].Selected, found = true, true
			}
		}
		if !found {
			return Node{}, nil, &Error{fmt.Sprintf("%s[%d]", p.field("value"), n), fmt.Sprintf("%q is not the value of any of the options", value)}
		}
	}
	return Node{Label: label, Variant: variant, Multiple: multiple, Options: options}, nil, nil
}
