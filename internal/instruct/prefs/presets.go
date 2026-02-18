package prefs

// PresetDefaults returns a fully populated Preferences struct for the given
// preset. Unknown preset names return pragmatic defaults.
func PresetDefaults(preset PresetName) *Preferences {
	switch preset {
	case PresetStrict:
		return strictDefaults()
	case PresetStartup:
		return startupDefaults()
	case PresetPragmatic:
		return pragmaticDefaults()
	default:
		return pragmaticDefaults()
	}
}

func strictDefaults() *Preferences {
	return &Preferences{
		Version: CurrentVersion,
		Preset:  PresetStrict,
		Testing: TestingPrefs{
			Style:          TestingTDD,
			CoverageTarget: 90,
			MockApproach:   "interfaces",
			TableDriven:    true,
		},
		ErrorHandling: ErrorHandlingPrefs{
			Style:       ErrorCustom,
			EarlyReturn: true,
			WrapContext:  true,
		},
		Organization: OrganizationPrefs{
			Style:       OrgClean,
			FileNaming:  "snake_case",
			InternalPkg: true,
		},
		Naming: NamingPrefs{
			ReceiverStyle: "abbreviated",
			InterfaceEr:   true,
			Abbreviations: "ID not Id, URL not Url",
		},
		Abstraction: AbstractionPrefs{
			MaxFunctionLines:   30,
			ExtractThreshold:   15,
			GenericsPolicy:     "when-obvious",
			InterfaceOwnership: "consumer",
		},
		Documentation: DocumentationPrefs{
			GodocRequired:  true,
			CommentStyle:   "why-not-what",
			ReadmeRequired: true,
		},
		Formatting: FormattingPrefs{
			LineLengthLimit: 120,
			ImportOrder:     []string{"stdlib", "external", "internal"},
			LinterConfig:    "strict",
		},
		Dependencies: DependencyPrefs{
			Philosophy: "minimal",
			Vendoring:  false,
		},
		Architecture: ArchitecturePrefs{
			LayerStructure: "layered",
			DepDirection:   "inward",
		},
	}
}

func pragmaticDefaults() *Preferences {
	return &Preferences{
		Version: CurrentVersion,
		Preset:  PresetPragmatic,
		Testing: TestingPrefs{
			Style:          TestingTestAfter,
			CoverageTarget: 70,
			MockApproach:   "interfaces",
			TableDriven:    true,
		},
		ErrorHandling: ErrorHandlingPrefs{
			Style:       ErrorWrapping,
			EarlyReturn: true,
			WrapContext:  true,
		},
		Organization: OrganizationPrefs{
			Style:       OrgFlat,
			FileNaming:  "snake_case",
			InternalPkg: true,
		},
		Naming: NamingPrefs{
			ReceiverStyle: "single-letter",
			InterfaceEr:   true,
			Abbreviations: "ID not Id",
		},
		Abstraction: AbstractionPrefs{
			MaxFunctionLines:   50,
			ExtractThreshold:   25,
			GenericsPolicy:     "when-obvious",
			InterfaceOwnership: "consumer",
		},
		Documentation: DocumentationPrefs{
			GodocRequired:  false,
			CommentStyle:   "why-not-what",
			ReadmeRequired: false,
		},
		Formatting: FormattingPrefs{
			LineLengthLimit: 120,
			ImportOrder:     []string{"stdlib", "external", "internal"},
			LinterConfig:    "standard",
		},
		Dependencies: DependencyPrefs{
			Philosophy: "pragmatic",
			Vendoring:  false,
		},
		Architecture: ArchitecturePrefs{
			LayerStructure: "flat",
			DepDirection:   "inward",
		},
	}
}

func startupDefaults() *Preferences {
	return &Preferences{
		Version: CurrentVersion,
		Preset:  PresetStartup,
		Testing: TestingPrefs{
			Style:          TestingMinimal,
			CoverageTarget: 40,
			MockApproach:   "minimal",
			TableDriven:    false,
		},
		ErrorHandling: ErrorHandlingPrefs{
			Style:       ErrorWrapping,
			EarlyReturn: true,
			WrapContext:  false,
		},
		Organization: OrganizationPrefs{
			Style:       OrgFlat,
			FileNaming:  "snake_case",
			InternalPkg: false,
		},
		Naming: NamingPrefs{
			ReceiverStyle: "single-letter",
			InterfaceEr:   false,
			Abbreviations: "",
		},
		Abstraction: AbstractionPrefs{
			MaxFunctionLines:   80,
			ExtractThreshold:   40,
			GenericsPolicy:     "avoid",
			InterfaceOwnership: "provider",
		},
		Documentation: DocumentationPrefs{
			GodocRequired:  false,
			CommentStyle:   "minimal",
			ReadmeRequired: false,
		},
		Formatting: FormattingPrefs{
			LineLengthLimit: 140,
			ImportOrder:     []string{"stdlib", "external", "internal"},
			LinterConfig:    "minimal",
		},
		Dependencies: DependencyPrefs{
			Philosophy: "batteries-included",
			Vendoring:  false,
		},
		Architecture: ArchitecturePrefs{
			LayerStructure: "flat",
			DepDirection:   "downward",
		},
	}
}
