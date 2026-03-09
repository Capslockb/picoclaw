package agent

import "testing"

func TestSanitizeProfilePathPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "telegram:12345", want: "telegram_12345"},
		{input: "  Alice/Bob  ", want: "alice_bob"},
		{input: "..", want: "anonymous"},
	}

	for _, tt := range tests {
		if got := sanitizeProfilePathPart(tt.input); got != tt.want {
			t.Fatalf("sanitizeProfilePathPart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
