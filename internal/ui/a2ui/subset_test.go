package a2ui

import (
	"slices"
	"testing"
)

func TestSubsetIsTheFloorAGrillingRoundNeeds(t *testing.T) {
	want := []string{"Button", "Card", "CheckBox", "ChoicePicker", "Column", "Divider", "List", "Row", "Text", "TextField"}
	if got := Subset(); !slices.Equal(got, want) {
		t.Fatalf("Subset() = %v, want %v", got, want)
	}
}
