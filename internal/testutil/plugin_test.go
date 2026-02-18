package testutil_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/testutil"
)

func TestScenarioPlugin_DetectReady(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")

	tests := []struct {
		content string
		want    bool
	}{
		{testutil.ContentReady, true},
		{testutil.ContentBusy, false},
		{testutil.ContentError, false},
		{"", false},
		{"> extra text", false},
	}

	for _, tt := range tests {
		got := p.DetectReady(tt.content)
		if got != tt.want {
			t.Errorf("DetectReady(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestScenarioPlugin_DetectBusy(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")

	tests := []struct {
		content string
		want    bool
	}{
		{testutil.ContentBusy, true},
		{testutil.ContentReady, false},
		{"still thinking about it", true},
		{"", false},
	}

	for _, tt := range tests {
		got := p.DetectBusy(tt.content)
		if got != tt.want {
			t.Errorf("DetectBusy(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestScenarioPlugin_DetectError(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")

	tests := []struct {
		content string
		wantMsg string
		wantErr bool
	}{
		{testutil.ContentError, "fatal", true},
		{"Error: something broke", "something broke", true},
		{testutil.ContentReady, "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		msg, isErr := p.DetectError(tt.content)
		if isErr != tt.wantErr {
			t.Errorf("DetectError(%q): isError = %v, want %v", tt.content, isErr, tt.wantErr)
		}
		if msg != tt.wantMsg {
			t.Errorf("DetectError(%q): msg = %q, want %q", tt.content, msg, tt.wantMsg)
		}
	}
}

func TestScenarioPlugin_DetectRateLimit(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")

	dur, limited := p.DetectRateLimit(testutil.ContentRateLimit)
	if !limited {
		t.Error("expected rate limit detection for sentinel content")
	}
	if dur <= 0 {
		t.Errorf("expected positive retry duration, got %v", dur)
	}

	_, limited = p.DetectRateLimit(testutil.ContentReady)
	if limited {
		t.Error("unexpected rate limit detection for ready content")
	}
}

func TestScenarioPlugin_ParseOutput_Default(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")

	output, err := p.ParseOutput("some content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.IsComplete {
		t.Error("expected IsComplete = true")
	}
	if output.Raw != "some content" {
		t.Errorf("Raw = %q, want %q", output.Raw, "some content")
	}
}

func TestScenarioPlugin_ParseOutput_Custom(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")
	p.SetParseOutputFn(func(content string) (plugin.AgentOutput, error) {
		return plugin.AgentOutput{
			Raw:          content,
			IsComplete:   true,
			FilesChanged: []string{"foo.go"},
		}, nil
	})

	output, err := p.ParseOutput("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.FilesChanged) != 1 || output.FilesChanged[0] != "foo.go" {
		t.Errorf("FilesChanged = %v, want [foo.go]", output.FilesChanged)
	}
}

func TestScenarioPlugin_Name(t *testing.T) {
	p := testutil.NewScenarioPlugin("my-plugin")
	if p.Name() != "my-plugin" {
		t.Errorf("Name() = %q, want %q", p.Name(), "my-plugin")
	}
}

func TestScenarioPlugin_LaunchCmd(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")
	bin, args, err := p.LaunchCmd(plugin.AgentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bin != "echo" {
		t.Errorf("bin = %q, want %q", bin, "echo")
	}
	if len(args) != 1 || args[0] != "mock" {
		t.Errorf("args = %v, want [mock]", args)
	}
}

func TestScenarioPlugin_Capabilities(t *testing.T) {
	p := testutil.NewScenarioPlugin("test")
	caps := p.Capabilities()
	if len(caps) < 2 {
		t.Fatalf("expected at least 2 capabilities, got %d", len(caps))
	}
}
