package controller

import "testing"

func TestMasker(t *testing.T) {
	tests := []struct {
		name    string
		secrets map[string]string
		masks   []string
		in      string
		want    string
	}{
		{
			name: "empty inputs pass through",
			in:   "nothing to hide",
			want: "nothing to hide",
		},
		{
			name:    "secret value is masked",
			secrets: map[string]string{"TOKEN": "s3cr3t"},
			in:      "auth=s3cr3t end",
			want:    "auth=*** end",
		},
		{
			name:    "multiple occurrences masked",
			secrets: map[string]string{"T": "abc"},
			in:      "abc-abc",
			want:    "***-***",
		},
		{
			name:  "runtime mask is applied",
			masks: []string{"runtime-secret"},
			in:    "x=runtime-secret",
			want:  "x=***",
		},
		{
			name:    "empty secret value is ignored (no over-masking)",
			secrets: map[string]string{"EMPTY": ""},
			in:      "unchanged",
			want:    "unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMasker(tt.secrets, tt.masks)
			if got := m.mask(tt.in); got != tt.want {
				t.Errorf("mask(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
