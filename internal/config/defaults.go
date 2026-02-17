package config

// DefaultConfig returns a Config populated with sensible zero-configuration
// defaults. The returned config passes Validate without modification.
func DefaultConfig() *Config {
	return &Config{
		Project: ProjectConfig{
			Name:     "my-project",
			Root:     ".",
			StateDir: ".crux",
		},
		Agents: map[string]AgentConfig{},
		Memory: MemoryConfig{
			SQLitePath:        ".crux/memory.db",
			VectorDir:         ".crux/vectors",
			EmbeddingProvider: "chromem-default",
			EmbeddingModel:    "nomic-embed-text",
		},
		Phases: PhaseConfig{
			SpecDir: "docs/phases",
		},
		Security: SecurityConfig{
			AuditLog:           ".crux/audit.log",
			MaxCmdsPerMin:      60,
			MaxFilesPerSession: 100,
			DeniedPaths:        []string{".git/", ".env", ".crux/audit.log"},
		},
		Context: ContextConfig{
			TotalBudget: 8000,
			WorldState:  300,
			DecisionRAG: 1500,
			Summary:     3000,
			Reserve:     3200,
		},
	}
}
