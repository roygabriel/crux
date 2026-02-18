package prefs

import (
	"testing"
)

func TestPresetDefaults_AllPresetsPopulateAllFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		preset PresetName
	}{
		{"strict", PresetStrict},
		{"pragmatic", PresetPragmatic},
		{"startup", PresetStartup},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := PresetDefaults(tt.preset)

			if p.Version != CurrentVersion {
				t.Errorf("Version = %q, want %q", p.Version, CurrentVersion)
			}
			if p.Preset != tt.preset {
				t.Errorf("Preset = %q, want %q", p.Preset, tt.preset)
			}

			// Testing
			if p.Testing.Style == "" {
				t.Error("Testing.Style is empty")
			}
			if p.Testing.CoverageTarget == 0 {
				t.Error("Testing.CoverageTarget is zero")
			}
			if p.Testing.MockApproach == "" {
				t.Error("Testing.MockApproach is empty")
			}

			// ErrorHandling
			if p.ErrorHandling.Style == "" {
				t.Error("ErrorHandling.Style is empty")
			}

			// Organization
			if p.Organization.Style == "" {
				t.Error("Organization.Style is empty")
			}
			if p.Organization.FileNaming == "" {
				t.Error("Organization.FileNaming is empty")
			}

			// Naming
			if p.Naming.ReceiverStyle == "" {
				t.Error("Naming.ReceiverStyle is empty")
			}

			// Abstraction
			if p.Abstraction.MaxFunctionLines == 0 {
				t.Error("Abstraction.MaxFunctionLines is zero")
			}
			if p.Abstraction.ExtractThreshold == 0 {
				t.Error("Abstraction.ExtractThreshold is zero")
			}
			if p.Abstraction.GenericsPolicy == "" {
				t.Error("Abstraction.GenericsPolicy is empty")
			}
			if p.Abstraction.InterfaceOwnership == "" {
				t.Error("Abstraction.InterfaceOwnership is empty")
			}

			// Documentation
			if p.Documentation.CommentStyle == "" {
				t.Error("Documentation.CommentStyle is empty")
			}

			// Formatting
			if p.Formatting.LineLengthLimit == 0 {
				t.Error("Formatting.LineLengthLimit is zero")
			}
			if len(p.Formatting.ImportOrder) == 0 {
				t.Error("Formatting.ImportOrder is empty")
			}
			if p.Formatting.LinterConfig == "" {
				t.Error("Formatting.LinterConfig is empty")
			}

			// Dependencies
			if p.Dependencies.Philosophy == "" {
				t.Error("Dependencies.Philosophy is empty")
			}

			// Architecture
			if p.Architecture.LayerStructure == "" {
				t.Error("Architecture.LayerStructure is empty")
			}
			if p.Architecture.DepDirection == "" {
				t.Error("Architecture.DepDirection is empty")
			}
		})
	}
}

func TestPresetDefaults_UnknownReturnsPragmatic(t *testing.T) {
	t.Parallel()

	unknown := PresetDefaults("nonexistent")
	pragmatic := PresetDefaults(PresetPragmatic)

	if unknown.Testing.CoverageTarget != pragmatic.Testing.CoverageTarget {
		t.Errorf("unknown CoverageTarget = %d, want %d",
			unknown.Testing.CoverageTarget, pragmatic.Testing.CoverageTarget)
	}
	if unknown.Abstraction.MaxFunctionLines != pragmatic.Abstraction.MaxFunctionLines {
		t.Errorf("unknown MaxFunctionLines = %d, want %d",
			unknown.Abstraction.MaxFunctionLines, pragmatic.Abstraction.MaxFunctionLines)
	}
	if unknown.Dependencies.Philosophy != pragmatic.Dependencies.Philosophy {
		t.Errorf("unknown Philosophy = %q, want %q",
			unknown.Dependencies.Philosophy, pragmatic.Dependencies.Philosophy)
	}
}

func TestPresetDefaults_DistinctValues(t *testing.T) {
	t.Parallel()

	strict := PresetDefaults(PresetStrict)
	pragmatic := PresetDefaults(PresetPragmatic)
	startup := PresetDefaults(PresetStartup)

	if strict.Testing.CoverageTarget != 90 {
		t.Errorf("strict CoverageTarget = %d, want 90", strict.Testing.CoverageTarget)
	}
	if pragmatic.Testing.CoverageTarget != 70 {
		t.Errorf("pragmatic CoverageTarget = %d, want 70", pragmatic.Testing.CoverageTarget)
	}
	if startup.Testing.CoverageTarget != 40 {
		t.Errorf("startup CoverageTarget = %d, want 40", startup.Testing.CoverageTarget)
	}

	if strict.Abstraction.MaxFunctionLines != 30 {
		t.Errorf("strict MaxFunctionLines = %d, want 30", strict.Abstraction.MaxFunctionLines)
	}
	if pragmatic.Abstraction.MaxFunctionLines != 50 {
		t.Errorf("pragmatic MaxFunctionLines = %d, want 50", pragmatic.Abstraction.MaxFunctionLines)
	}
	if startup.Abstraction.MaxFunctionLines != 80 {
		t.Errorf("startup MaxFunctionLines = %d, want 80", startup.Abstraction.MaxFunctionLines)
	}
}

func TestPreferences_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    Preferences
		want bool
	}{
		{
			name: "zero value",
			p:    Preferences{},
			want: true,
		},
		{
			name: "only version set",
			p:    Preferences{Version: CurrentVersion},
			want: false,
		},
		{
			name: "only preset set",
			p:    Preferences{Preset: PresetPragmatic},
			want: false,
		},
		{
			name: "fully populated",
			p:    *PresetDefaults(PresetPragmatic),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.p.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPreferences_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(*Preferences)
		wantErr string
	}{
		{
			name:   "valid pragmatic",
			modify: func(_ *Preferences) {},
		},
		{
			name:   "valid strict",
			modify: func(p *Preferences) { p.Preset = PresetStrict },
		},
		{
			name:   "valid startup",
			modify: func(p *Preferences) { p.Preset = PresetStartup },
		},
		{
			name:    "bad version",
			modify:  func(p *Preferences) { p.Version = "999.0" },
			wantErr: "unsupported preferences version",
		},
		{
			name:    "empty version",
			modify:  func(p *Preferences) { p.Version = "" },
			wantErr: "unsupported preferences version",
		},
		{
			name:    "bad preset",
			modify:  func(p *Preferences) { p.Preset = "unknown" },
			wantErr: "invalid preset name",
		},
		{
			name:    "empty preset",
			modify:  func(p *Preferences) { p.Preset = "" },
			wantErr: "invalid preset name",
		},
		{
			name:    "coverage too high",
			modify:  func(p *Preferences) { p.Testing.CoverageTarget = 101 },
			wantErr: "coverage_target must be between 0 and 100",
		},
		{
			name:    "coverage negative",
			modify:  func(p *Preferences) { p.Testing.CoverageTarget = -1 },
			wantErr: "coverage_target must be between 0 and 100",
		},
		{
			name:   "coverage zero is valid",
			modify: func(p *Preferences) { p.Testing.CoverageTarget = 0 },
		},
		{
			name:   "coverage 100 is valid",
			modify: func(p *Preferences) { p.Testing.CoverageTarget = 100 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := PresetDefaults(PresetPragmatic)
			tt.modify(p)
			err := p.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("Validate() error = %q, want containing %q", got, tt.wantErr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
