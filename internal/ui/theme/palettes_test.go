package theme

import "testing"

func TestEveryThemeHandsOutTheSameSwatches(t *testing.T) {
	want := len(Palettes[0].Swatches())
	for _, theme := range Palettes {
		swatches := theme.Swatches()
		if len(swatches) != want {
			t.Errorf("%s has %d swatches, want %d", theme.Name, len(swatches), want)
		}
		for i, color := range swatches {
			if color == "" {
				t.Errorf("%s swatch %d is empty", theme.Name, i)
			}
		}
	}
}
