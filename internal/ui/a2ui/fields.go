package a2ui

// answers is the components the owner answers: the ones a form focuses,
// steps and types into.
var answers = map[string]bool{"TextField": true, "CheckBox": true, "ChoicePicker": true}

// Fields lists the components the owner answers, in document order, as
// pointers into doc so a form changes the document it draws.
func Fields(doc *Document) []*Node {
	var fields []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		if !n.Gap && answers[n.Type] {
			fields = append(fields, n)
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(&doc.Root)
	return fields
}
