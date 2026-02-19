package orchestrator

import "testing"

func TestLooksLikePasteConfirmEcho(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "pasted content marker",
			content: "[Pasted Content 6803 chars]",
			want:    true,
		},
		{
			name:    "enter confirms marker",
			content: "message; Enter confirms.",
			want:    true,
		},
		{
			name:    "press enter marker",
			content: "Press Enter to confirm this paste",
			want:    true,
		},
		{
			name:    "normal ready screen",
			content: ">_ OpenAI Codex",
			want:    false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := looksLikePasteConfirmEcho(tt.content)
			if got != tt.want {
				t.Fatalf("looksLikePasteConfirmEcho(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
