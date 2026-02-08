package evaluator

import (
	"testing"
)

func TestCountFormatVerbs(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"", 0, false},
		{"Hello World", 0, false},
		{"Hello %s", 1, false},
		{"%d", 1, false},
		{"%d %s", 2, false},
		{"%%", 0, false},
		{"%% %d", 1, false},
		{"%d %% %s", 2, false},
		{"%.2f", 1, false},
		{"%5d", 1, false},
		{"%05d", 1, false},
		{"%-5d", 1, false},
		{"%+d", 1, false},
		{"%#x", 1, false},
		{"% 5d", 1, false}, // Space flag
		{"%v", 1, false},
		{"%t", 1, false},
		{"%b", 1, false},
		{"%c", 1, false},
		{"%o", 1, false},
		{"%O", 1, false},
		{"%q", 1, false},
		{"%x", 1, false},
		{"%X", 1, false},
		{"%e", 1, false},
		{"%E", 1, false},
		{"%g", 1, false},
		{"%G", 1, false},
		{"%p", 1, false},
		{"%U", 1, false},

		// Invalid cases
		{"%", 0, true},    // Unterminated
		{"%z", 0, true},   // Invalid verb
		{"%d %", 0, true}, // Unterminated at end
		{"%.", 0, true},   // Unterminated precision
		{"%5", 0, true},   // Unterminated width
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := CountFormatVerbs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CountFormatVerbs(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("CountFormatVerbs(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
