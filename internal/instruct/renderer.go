package instruct

import (
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"text/template"
)

// SectionMeta describes a template section's name, priority, and display order.
type SectionMeta struct {
	// Name is the template name (e.g., "identity").
	Name string `json:"name"`
	// Priority controls drop order when over budget.
	Priority SectionPriority `json:"priority"`
	// Order is the display position in the final output.
	Order int `json:"order"`
}

// DefaultSections returns the section registry in display order.
func DefaultSections() []SectionMeta {
	return []SectionMeta{
		{Name: "identity", Priority: PriorityCritical, Order: 0},
		{Name: "project", Priority: PriorityCritical, Order: 1},
		{Name: "responsibilities", Priority: PriorityCritical, Order: 2},
		{Name: "constraints", Priority: PriorityCritical, Order: 3},
		{Name: "preferences", Priority: PriorityHigh, Order: 4},
		{Name: "phase", Priority: PriorityHigh, Order: 5},
		{Name: "memory", Priority: PriorityMedium, Order: 6},
		{Name: "mcp", Priority: PriorityMedium, Order: 7},
		{Name: "skills", Priority: PriorityLow, Order: 8},
		{Name: "team", Priority: PriorityLow, Order: 9},
		{Name: "session", Priority: PriorityCritical, Order: 10},
	}
}

// RenderedSection holds the output of rendering a single template section.
type RenderedSection struct {
	// Name is the section template name.
	Name string `json:"name"`
	// Content is the rendered markdown text.
	Content string `json:"content"`
	// Priority is the section's drop priority.
	Priority SectionPriority `json:"priority"`
	// Tokens is the estimated token count of the rendered content.
	Tokens int `json:"tokens"`
}

// RenderResult holds the complete output of a Render call.
type RenderResult struct {
	// Content is the final concatenated markdown.
	Content string `json:"content"`
	// Sections lists all sections that were included.
	Sections []RenderedSection `json:"sections"`
	// TotalTokens is the total token count of the final output.
	TotalTokens int `json:"total_tokens"`
	// Dropped lists section names that were dropped to meet the budget.
	Dropped []string `json:"dropped,omitempty"`
	// Warnings lists non-fatal issues encountered during rendering.
	Warnings []string `json:"warnings,omitempty"`
}

// Renderer parses and renders instruction templates with token budget enforcement.
type Renderer struct {
	templates *template.Template
	sections  []SectionMeta
	logger    *slog.Logger
}

// NewRenderer creates a Renderer by parsing all .tmpl files from the given
// filesystem. Templates use [[ ]] delimiters to avoid conflicts with Go
// code blocks in rendered markdown.
func NewRenderer(templateFS fs.FS, logger *slog.Logger) (*Renderer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	tmpl := template.New("").Delims("[[", "]]").Funcs(DefaultFuncMap())

	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		data, readErr := fs.ReadFile(templateFS, path)
		if readErr != nil {
			return fmt.Errorf("reading template %s: %w", path, readErr)
		}

		// Derive template name from filename without extension.
		name := strings.TrimSuffix(d.Name(), ".md.tmpl")
		if _, parseErr := tmpl.New(name).Parse(string(data)); parseErr != nil {
			return fmt.Errorf("parsing template %s: %w", path, parseErr)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return &Renderer{
		templates: tmpl,
		sections:  DefaultSections(),
		logger:    logger,
	}, nil
}

// Render generates the final instruction content by rendering each section,
// enforcing the token budget by dropping low-priority sections first, and
// concatenating the result in display order.
func (r *Renderer) Render(data InstructionData, budget int) (*RenderResult, error) {
	// Render all sections independently.
	rendered := make([]RenderedSection, 0, len(r.sections))
	for _, meta := range r.sections {
		section, err := r.renderOneSection(meta.Name, data)
		if err != nil {
			r.logger.Warn("failed to render section", "section", meta.Name, "error", err)
			continue
		}

		// Skip sections that produce only whitespace.
		if strings.TrimSpace(section.Content) == "" {
			continue
		}

		section.Priority = meta.Priority
		rendered = append(rendered, *section)
	}

	// If budget <= 0, include only critical sections.
	if budget <= 0 {
		return r.assembleCriticalOnly(rendered)
	}

	// Calculate total tokens.
	totalTokens := 0
	for _, s := range rendered {
		totalTokens += s.Tokens
	}

	// If under budget, include everything.
	if totalTokens <= budget {
		return r.assemble(rendered, nil), nil
	}

	// Over budget: drop sections by priority (Low first, then Medium, then High).
	// Critical sections are never dropped.
	return r.dropToBudget(rendered, budget)
}

// RenderSection renders a single named section for testing or preview.
func (r *Renderer) RenderSection(name string, data InstructionData) (*RenderedSection, error) {
	return r.renderOneSection(name, data)
}

// RenderTemplate executes a named template with the given data and returns
// the rendered content. Unlike Render(), it does not apply section-based
// budget enforcement.
func (r *Renderer) RenderTemplate(name string, data InstructionData) (string, error) {
	tmpl := r.templates.Lookup(name)
	if tmpl == nil {
		return "", fmt.Errorf("template %q not found", name)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}
	return buf.String(), nil
}

func (r *Renderer) renderOneSection(name string, data InstructionData) (*RenderedSection, error) {
	tmpl := r.templates.Lookup(name)
	if tmpl == nil {
		return nil, fmt.Errorf("template %q not found", name)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", name, err)
	}

	content := buf.String()
	return &RenderedSection{
		Name:    name,
		Content: content,
		Tokens:  EstimateTokens(content),
	}, nil
}

func (r *Renderer) assembleCriticalOnly(rendered []RenderedSection) (*RenderResult, error) {
	var included []RenderedSection
	var dropped []string

	for _, s := range rendered {
		if s.Priority == PriorityCritical {
			included = append(included, s)
		} else {
			dropped = append(dropped, s.Name)
		}
	}

	result := r.assemble(included, dropped)
	if len(dropped) > 0 {
		result.Warnings = append(result.Warnings, "zero budget: only critical sections included")
	}
	return result, nil
}

func (r *Renderer) dropToBudget(rendered []RenderedSection, budget int) (*RenderResult, error) {
	// Build a list sorted by priority descending (highest priority number = drop first).
	type indexedSection struct {
		index   int
		section RenderedSection
	}

	// Separate into critical (never drop) and droppable.
	var critical []RenderedSection
	var droppable []indexedSection
	criticalTokens := 0

	for i, s := range rendered {
		if s.Priority == PriorityCritical {
			critical = append(critical, s)
			criticalTokens += s.Tokens
		} else {
			droppable = append(droppable, indexedSection{index: i, section: s})
		}
	}

	// Sort droppable by priority descending (Low=3 first), stable by order.
	sort.SliceStable(droppable, func(i, j int) bool {
		return droppable[i].section.Priority > droppable[j].section.Priority
	})

	// Drop from the front (lowest priority) until under budget.
	remaining := budget - criticalTokens
	var dropped []string
	var included []indexedSection

	for _, ds := range droppable {
		if remaining >= ds.section.Tokens {
			included = append(included, ds)
			remaining -= ds.section.Tokens
		} else {
			dropped = append(dropped, ds.section.Name)
			r.logger.Info("dropped section for budget",
				"section", ds.section.Name,
				"tokens", ds.section.Tokens,
				"priority", ds.section.Priority.String(),
			)
		}
	}

	// Re-sort included by original index to maintain display order.
	sort.SliceStable(included, func(i, j int) bool {
		return included[i].index < included[j].index
	})

	// Merge critical + included in display order.
	// Build a map of original index to section for ordering.
	all := make([]RenderedSection, 0, len(critical)+len(included))

	// We need to interleave critical and droppable by their original section order.
	// Build index map from section meta.
	orderMap := make(map[string]int, len(r.sections))
	for _, meta := range r.sections {
		orderMap[meta.Name] = meta.Order
	}

	all = append(all, critical...)
	for _, ds := range included {
		all = append(all, ds.section)
	}

	sort.SliceStable(all, func(i, j int) bool {
		return orderMap[all[i].Name] < orderMap[all[j].Name]
	})

	result := r.assemble(all, dropped)
	if len(dropped) > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("dropped %d section(s) to meet token budget of %d", len(dropped), budget))
	}
	return result, nil
}

func (r *Renderer) assemble(sections []RenderedSection, dropped []string) *RenderResult {
	var buf strings.Builder
	totalTokens := 0

	for _, s := range sections {
		buf.WriteString(s.Content)
		totalTokens += s.Tokens
	}

	return &RenderResult{
		Content:     buf.String(),
		Sections:    sections,
		TotalTokens: totalTokens,
		Dropped:     dropped,
	}
}
