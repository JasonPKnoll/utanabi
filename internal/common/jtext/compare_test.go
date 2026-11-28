package jtext

import "testing"

func TestPlain(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{in: "Shinunoga E-Wa", want: "shinunogaewa"},
		{in: "うっせぇわ！", want: "うっせぇわ"},
		{in: "ＡＢＣ　１２", want: "abc12"},
	}
	for _, tt := range tests {
		if got := Plain(tt.in); got != tt.want {
			t.Errorf("Plain(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFoldRomaji(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{in: "Tōkyō Jihen", want: "tokyojihen"},
		{in: "toukyou jihen", want: "tokyojihen"},
		{in: "Tokyo Jihen", want: "tokyojihen"},
		{in: "Ookii", want: "oki"},
		{in: "Shiseikatsu", want: "shiseikatsu"},
	}
	for _, tt := range tests {
		if got := FoldRomaji(tt.in); got != tt.want {
			t.Errorf("FoldRomaji(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
