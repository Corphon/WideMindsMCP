// LLM Scheduler

package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"WideMindsMCP/internal/models"
)

// Struct definitions
type LLMOrchestrator struct {
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
}

type LLMRequest struct {
	Prompt      string
	Context     []string
	Temperature float64
	MaxTokens   int
}

type LLMResponse struct {
	Content   string
	Usage     TokenUsage
	Model     string
	Timestamp time.Time
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type promptTemplate struct {
	role         string
	mission      string
	deliverables []string
	constraints  []string
	reasoning    []string
	styleNotes   []string
	examples     []fewShotExample
	closing      string
}

type fewShotExample struct {
	name   string
	input  string
	output string
}

type promptContextSegments struct {
	background  []string
	history     []string
	preferences []string
	goals       []string
	additional  []string
}

// Constructors
func NewLLMOrchestrator(apiKey, baseURL, model string) *LLMOrchestrator {
	if model == "" {
		model = "gpt-4.1"
	}

	return &LLMOrchestrator{
		apiKey:    apiKey,
		baseURL:   baseURL,
		model:     model,
		maxTokens: 32768,
	}
}

// Methods
func (llm *LLMOrchestrator) GenerateThoughtDirections(concept string, context []string) ([]models.Direction, error) {
	if concept == "" {
		return nil, errors.New("concept is required")
	}

	normalizedContext := make([]string, 0, len(context))
	for _, entry := range context {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			normalizedContext = append(normalizedContext, trimmed)
		}
	}

	prompt := llm.BuildPrompt(concept, normalizedContext, "directions")
	_ = prompt // placeholder for future real LLM call

	baseKeywords := strings.Fields(strings.ToLower(concept))
	keywords := append(normalizedContext, baseKeywords...)

	directionTemplates := []struct {
		dirType models.DirectionType
		title   string
		descFmt string
	}{
		{models.Broad, "Macro Overview", "Map the overall structure and key components of %v from a macro perspective."},
		{models.Deep, "Technical Deep Dive", "Explore the critical technical details and underlying principles of %v in depth."},
		{models.Lateral, "Cross-domain Associations", "Identify lateral connections between %v and other disciplines, industries, or mental models."},
		{models.Critical, "Critical Reflection", "Assess the potential risks, blind spots, and areas for improvement when applying %v in practice."},
	}

	directions := make([]models.Direction, 0, len(directionTemplates))
	for i, tpl := range directionTemplates {
		desc := fmt.Sprintf(tpl.descFmt, concept)
		dir := models.Direction{
			Type:        tpl.dirType,
			Title:       tpl.title,
			Description: desc,
			Keywords:    uniqueStrings(keywords),
			Relevance:   math.Max(0.3, 1.0-0.1*float64(i)),
		}
		directions = append(directions, dir)
	}

	return directions, nil
}

func (llm *LLMOrchestrator) ExploreDirection(direction models.Direction, depth int) ([]*models.Thought, error) {
	if depth <= 0 {
		depth = 1
	}

	thoughts := make([]*models.Thought, 0, depth)
	for i := 0; i < depth; i++ {
		content := fmt.Sprintf("%s - depth level %d", direction.Title, i+1)
		thought := models.NewThought(content, "", direction)
		thought.Depth = i + 1
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

func (llm *LLMOrchestrator) CallLLM(req *LLMRequest) (*LLMResponse, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if req.Prompt == "" {
		return nil, errors.New("prompt is empty")
	}

	if req.MaxTokens == 0 {
		req.MaxTokens = llm.maxTokens
	}

	summary := truncate(req.Prompt, 512)

	return &LLMResponse{
		Content: summary,
		Usage: TokenUsage{
			PromptTokens:     len(strings.Fields(req.Prompt)),
			CompletionTokens: len(strings.Fields(summary)),
			TotalTokens:      len(strings.Fields(req.Prompt)) + len(strings.Fields(summary)),
		},
		Model:     llm.model,
		Timestamp: time.Now().UTC(),
	}, nil
}

func (llm *LLMOrchestrator) HealthCheck(ctx context.Context) error {
	if llm == nil {
		return errors.New("llm orchestrator is nil")
	}
	_ = ctx
	return nil
}

func (llm *LLMOrchestrator) BuildPrompt(concept string, context []string, promptType string) string {
	tpl := llm.promptTemplateFor(promptType)
	data := map[string]string{
		"concept":    concept,
		"model":      llm.model,
		"promptType": promptType,
	}
	segments := extractContextSegments(context)

	var builder strings.Builder
	builder.Grow(1024)

	builder.WriteString(renderTemplate("System role: "+tpl.role+"\n\n", data))

	builder.WriteString("## Mission\n")
	builder.WriteString(renderTemplate(tpl.mission, data))
	builder.WriteString("\n\n")

	if len(segments.goals) > 0 {
		builder.WriteString("## Explicit user goals\n")
		writeNumberedList(&builder, segments.goals)
	} else {
		builder.WriteString("## Explicit user goals\n- Default goal: deepen understanding of the concept and surface actionable exploration directions.\n\n")
	}

	if len(segments.background) > 0 {
		builder.WriteString("## Background information\n")
		writeBulletedList(&builder, segments.background)
	}

	if len(segments.history) > 0 {
		builder.WriteString("## Historical path\n")
		writeBulletedList(&builder, segments.history)
	}

	if len(segments.preferences) > 0 {
		builder.WriteString("## User preferences\n")
		writeBulletedList(&builder, segments.preferences)
	}

	if len(segments.additional) > 0 {
		builder.WriteString("## Additional notes\n")
		writeBulletedList(&builder, segments.additional)
	}

	if len(tpl.deliverables) > 0 {
		builder.WriteString("## Output requirements\n")
		writeNumberedList(&builder, renderTemplateList(tpl.deliverables, data))
	}

	if len(tpl.constraints) > 0 {
		builder.WriteString("## Constraints\n")
		writeBulletedList(&builder, renderTemplateList(tpl.constraints, data))
	}

	if len(tpl.reasoning) > 0 {
		builder.WriteString("## Reasoning steps\n")
		writeNumberedList(&builder, renderTemplateList(tpl.reasoning, data))
	}

	if len(tpl.styleNotes) > 0 {
		builder.WriteString("## Style guidelines\n")
		writeBulletedList(&builder, renderTemplateList(tpl.styleNotes, data))
	}

	if len(tpl.examples) > 0 {
		builder.WriteString("## Reference examples\n")
		for i, example := range tpl.examples {
			number := i + 1
			builder.WriteString(fmt.Sprintf("### Example %d - %s\n", number, renderTemplate(example.name, data)))
			builder.WriteString("<Input>\n")
			builder.WriteString(strings.TrimSpace(renderTemplate(example.input, data)))
			builder.WriteString("\n<Output>\n")
			builder.WriteString(strings.TrimSpace(renderTemplate(example.output, data)))
			builder.WriteString("\n\n")
		}
	}

	builder.WriteString("## Output format\n")
	builder.WriteString("- Prefer structured JSON with a concise natural-language summary.\n")
	builder.WriteString("- Each direction in the JSON array must include type, title, summary, key_questions, and recommended_actions fields.\n")
	builder.WriteString("- If requirements cannot be met, explicitly state the missing information and suggest a next step.\n\n")

	if tpl.closing != "" {
		builder.WriteString(renderTemplate(tpl.closing, data))
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func (llm *LLMOrchestrator) promptTemplateFor(promptType string) promptTemplate {
	switch strings.ToLower(strings.TrimSpace(promptType)) {
	case "directions":
		return promptTemplate{
			role:    "You are an experienced learning-path architect and knowledge-graph advisor who excels at breaking abstract themes into complementary exploration directions.",
			mission: "Generate 3-5 expansion directions around the concept '{{concept}}' so the user can broaden their thinking while staying aligned with the provided context.",
			deliverables: []string{
				"For each direction return type (broad/deep/lateral/critical/other), title, summary, key_questions (>=3 items), and recommended_actions (>=2 items).",
				"Provide a direction_rationale that links the suggestion to the user's background or preferences.",
				"Conclude with next_step_recommendations to help the user choose or combine directions.",
			},
			constraints: []string{
				"Stay accurate and transparent; if information is missing, call it out.",
				"Ensure the directions are distinct and non-overlapping.",
				"All output must be in English; include original terminology in parentheses if it aids clarity.",
			},
			reasoning: []string{
				"Synthesize the background, history, and preferences to uncover the core intent.",
				"Use chain-of-thought reasoning to enumerate possible directions and weigh their value, risk, and prerequisites.",
				"Select the most representative directions and organize them into the requested structure.",
			},
			styleNotes: []string{
				"Keep the tone professional and encouraging, never condescending.",
				"Make the final summary concise and decision-oriented.",
			},
			examples: []fewShotExample{
				{
					name: "Machine learning concept expansion",
					input: `Concept: Machine Learning
Background: strong statistics foundation
History: completed "Statistical Learning Methods"
Preference: project-driven learning`,
					output: `[
  {
    "type": "broad",
    "title": "Algorithm landscape overview",
    "summary": "Construct a whole-picture view across supervised, unsupervised, and reinforcement learning paradigms.",
    "key_questions": [
      "Which canonical algorithms form the backbone of modern machine learning?",
      "What are the typical use cases and trade-offs between these algorithm families?",
      "How do data scale and noise characteristics influence algorithm selection?"
    ],
    "recommended_actions": [
      "Create a comparison table summarizing assumptions, inputs, and outputs of major algorithms.",
      "Select a public dataset, run at least two algorithm families, and record performance differences."
    ],
    "direction_rationale": "The user's statistics background accelerates understanding of algorithmic assumptions and trade-offs."
  }
]`,
				},
			},
			closing: "If critical information is missing, add an 'open_questions' field at the end listing what the user should clarify next.",
		}
	case "exploration":
		return promptTemplate{
			role:    "You are a seasoned research coach who guides users through deep exploration and validation.",
			mission: "For the concept '{{concept}}' and a chosen direction, deliver an actionable plan covering research outline, core ideas, and validation steps.",
			deliverables: []string{
				"Return hypothesis, key_concepts, resources, validation_steps, and reflection_questions fields.",
				"Each field should include at least 2-3 high-quality suggestions with rationale.",
			},
			constraints: []string{
				"Cite the type or credibility of any referenced resources; note when proof is lacking.",
				"Keep guidance concrete and actionable, avoiding vague descriptions.",
			},
			reasoning: []string{
				"Assess the user's current progress and gaps.",
				"Propose hypotheses and validation paths with feasible resources and steps.",
			},
			styleNotes: []string{
				"Use precise language that highlights action priorities.",
			},
			closing: "Finish with a 'checkpoints' list to help the user measure interim progress.",
		}
	default:
		return promptTemplate{
			role:    "You are a reliable knowledge-collaboration assistant.",
			mission: "Provide a structured analysis and actionable advice around the concept '{{concept}}'.",
			deliverables: []string{
				"Return summary, key_points, and next_actions fields.",
			},
			constraints: []string{
				"Maintain factual accuracy and call out assumptions when needed.",
			},
			closing: "If information is insufficient for action, list the questions the user should answer next.",
		}
	}
}

func extractContextSegments(entries []string) promptContextSegments {
	segments := promptContextSegments{}

	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		key := ""
		value := entry
		if idx := strings.Index(entry, ":"); idx >= 0 {
			key = strings.ToLower(strings.TrimSpace(entry[:idx]))
			value = strings.TrimSpace(entry[idx+1:])
		}

		switch key {
		case "background", "context", "domain":
			segments.background = append(segments.background, value)
		case "history", "path", "trajectory":
			segments.history = append(segments.history, value)
		case "preference", "preferences", "style", "tone":
			segments.preferences = append(segments.preferences, value)
		case "goal", "goals", "objective", "intent":
			segments.goals = append(segments.goals, value)
		default:
			if key != "" && value != "" {
				segments.additional = append(segments.additional, fmt.Sprintf("%s: %s", key, value))
			} else {
				segments.additional = append(segments.additional, value)
			}
		}
	}

	return segments
}

func renderTemplate(input string, data map[string]string) string {
	result := input
	for key, value := range data {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func renderTemplateList(items []string, data map[string]string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, renderTemplate(item, data))
	}
	return result
}

func writeNumberedList(builder *strings.Builder, items []string) {
	if len(items) == 0 {
		builder.WriteString("- None.\n\n")
		return
	}
	for i, item := range items {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(item)))
	}
	builder.WriteString("\n")
}

func writeBulletedList(builder *strings.Builder, items []string) {
	if len(items) == 0 {
		return
	}
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(item)))
	}
	builder.WriteString("\n")
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, v := range values {
		normalized := strings.TrimSpace(v)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func truncate(input string, max int) string {
	if len([]rune(input)) <= max {
		return input
	}
	runes := []rune(input)
	return string(runes[:max])
}
