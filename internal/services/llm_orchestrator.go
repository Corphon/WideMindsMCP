// LLM Scheduler

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"WideMindsMCP/internal/models"
	"WideMindsMCP/internal/utils"
)

// Struct definitions
type LLMOrchestrator struct {
	apiKey     string
	baseURL    string
	model      string
	maxTokens  int
	httpClient *http.Client
	timeout    time.Duration
}

func (llm *LLMOrchestrator) hasRemoteBackend() bool {
	return llm != nil && llm.baseURL != "" && llm.httpClient != nil
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
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		maxTokens:  32768,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		timeout:    15 * time.Second,
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
	if llm.hasRemoteBackend() {
		resp, err := llm.CallLLM(&LLMRequest{
			Prompt:      prompt,
			Context:     normalizedContext,
			Temperature: 0.7,
			MaxTokens:   1024,
		})
		if err != nil {
			utils.Warn("LLM call failed while generating directions", utils.KV("error", err))
		} else if resp != nil {
			if directions, parseErr := llm.parseDirectionsFromContent(resp.Content); parseErr != nil {
				utils.Warn("failed to parse LLM directions response", utils.KV("error", parseErr))
			} else if len(directions) > 0 {
				return directions, nil
			}
		}
	}

	return llm.generateFallbackDirections(concept, normalizedContext), nil
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
	if llm == nil {
		return nil, errors.New("llm orchestrator is nil")
	}

	if req == nil {
		return nil, errors.New("request is nil")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is empty")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = int(math.Min(float64(llm.maxTokens), 2048))
	} else if maxTokens > llm.maxTokens {
		maxTokens = llm.maxTokens
	}

	temperature := req.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}
	temperature = math.Max(0, math.Min(temperature, 2))

	if !llm.hasRemoteBackend() {
		return llm.localLLMResponse(prompt, maxTokens), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), llm.timeout)
	defer cancel()

	userContent := prompt
	if len(req.Context) > 0 {
		var sb strings.Builder
		sb.Grow(len(prompt) + 128)
		sb.WriteString(prompt)
		sb.WriteString("\n\nContext:\n")
		for _, entry := range uniqueStrings(req.Context) {
			sb.WriteString("- ")
			sb.WriteString(entry)
			sb.WriteString("\n")
		}
		userContent = strings.TrimSpace(sb.String())
	}

	payload := map[string]any{
		"model": llm.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an assistant that returns valid JSON matching the user's instructions."},
			{"role": "user", "content": userContent},
		},
		"max_tokens":  maxTokens,
		"temperature": temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal llm payload: %w", err)
	}

	endpoint := llm.baseURL
	if !strings.HasSuffix(endpoint, "/v1/chat/completions") {
		endpoint = strings.TrimRight(endpoint, "/") + "/v1/chat/completions"
	}

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new http request: %w", err)
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	if llm.apiKey != "" {
		reqHTTP.Header.Set("Authorization", "Bearer "+llm.apiKey)
	}

	resp, err := llm.httpClient.Do(reqHTTP)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}

	if resp.StatusCode >= 400 {
		snippet := truncate(string(raw), 512)
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, snippet)
	}

	var parsed struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode llm response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return nil, errors.New("llm response missing choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(parsed.Choices[0].Text)
	}
	if content == "" {
		return nil, errors.New("llm response empty")
	}

	usage := TokenUsage{
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
	}

	model := parsed.Model
	if model == "" {
		model = llm.model
	}

	return &LLMResponse{
		Content:   content,
		Usage:     usage,
		Model:     model,
		Timestamp: time.Now().UTC(),
	}, nil
}

func (llm *LLMOrchestrator) localLLMResponse(prompt string, maxTokens int) *LLMResponse {
	summary := truncate(prompt, maxTokens)
	promptTokens := len(strings.Fields(prompt))
	completionTokens := len(strings.Fields(summary))

	return &LLMResponse{
		Content: summary,
		Usage: TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
		Model:     llm.model,
		Timestamp: time.Now().UTC(),
	}
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

func (llm *LLMOrchestrator) parseDirectionsFromContent(content string) ([]models.Direction, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, errors.New("llm response empty")
	}

	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var raw []struct {
		Type                string   `json:"type"`
		Title               string   `json:"title"`
		Summary             string   `json:"summary"`
		Description         string   `json:"description"`
		DirectionRationale  string   `json:"direction_rationale"`
		KeyQuestions        []string `json:"key_questions"`
		RecommendedActions  []string `json:"recommended_actions"`
		Keywords            []string `json:"keywords"`
		Relevance           float64  `json:"relevance"`
		Confidence          float64  `json:"confidence"`
		Importance          float64  `json:"importance"`
		SuggestedRelevance  float64  `json:"suggested_relevance"`
		SuggestedConfidence float64  `json:"suggested_confidence"`
	}

	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("parse llm directions: %w", err)
	}

	results := make([]models.Direction, 0, len(raw))
	for _, item := range raw {
		title := strings.TrimSpace(item.Title)
		description := strings.TrimSpace(item.Description)
		if description == "" {
			description = strings.TrimSpace(item.Summary)
		}
		if title == "" || description == "" {
			continue
		}

		typeStr := strings.ToLower(strings.TrimSpace(item.Type))
		var dirType models.DirectionType
		switch typeStr {
		case string(models.Broad), "overview", "expansion":
			dirType = models.Broad
		case string(models.Deep), "deepen", "analysis":
			dirType = models.Deep
		case string(models.Lateral), "adjacent":
			dirType = models.Lateral
		case string(models.Critical), "challenge":
			dirType = models.Critical
		default:
			dirType = models.Broad
		}

		keywords := uniqueStrings(append(append([]string{}, item.Keywords...), item.KeyQuestions...))
		if len(keywords) == 0 && item.DirectionRationale != "" {
			keywords = append(keywords, truncate(item.DirectionRationale, 64))
		}
		keywords = uniqueStrings(keywords)

		relevance := item.Relevance
		if relevance == 0 {
			relevance = item.Confidence
		}
		if relevance == 0 {
			relevance = item.Importance
		}
		if relevance == 0 {
			relevance = item.SuggestedRelevance
		}
		if relevance == 0 {
			relevance = item.SuggestedConfidence
		}
		relevance = math.Max(0, math.Min(relevance, 1))
		if relevance == 0 {
			relevance = 0.7
		}

		direction := models.Direction{
			Type:        dirType,
			Title:       title,
			Description: description,
			Keywords:    keywords,
			Relevance:   relevance,
		}
		results = append(results, direction)
	}

	if len(results) == 0 {
		return nil, errors.New("no valid directions returned")
	}

	return results, nil
}

func (llm *LLMOrchestrator) generateFallbackDirections(concept string, context []string) []models.Direction {
	concept = strings.TrimSpace(concept)
	if concept == "" {
		concept = "the topic"
	}

	keyTopics := uniqueStrings(context)
	if len(keyTopics) == 0 {
		keyTopics = []string{concept}
	}

	baseRelevance := 0.65 + math.Min(float64(len(keyTopics))*0.03, 0.25)

	plans := []struct {
		dirType models.DirectionType
		title   string
		desc    string
		keys    []string
	}{
		{
			dirType: models.Broad,
			title:   fmt.Sprintf("Mapping the %s landscape", concept),
			desc:    fmt.Sprintf("Survey the primary themes, actors, and trends that define %s today.", concept),
			keys:    append([]string{"overview", concept}, keyTopics...),
		},
		{
			dirType: models.Deep,
			title:   fmt.Sprintf("Deep dive into core mechanics of %s", concept),
			desc:    fmt.Sprintf("Analyze foundational principles, frameworks, and edge cases that underpin %s.", concept),
			keys:    append([]string{"analysis", "core principles"}, keyTopics...),
		},
		{
			dirType: models.Lateral,
			title:   fmt.Sprintf("Adjacent inspirations for %s", concept),
			desc:    fmt.Sprintf("Explore parallels from neighboring domains to reframe assumptions about %s.", concept),
			keys:    append([]string{"analogy", "cross-domain"}, keyTopics...),
		},
		{
			dirType: models.Critical,
			title:   fmt.Sprintf("Stress-testing %s assumptions", concept),
			desc:    fmt.Sprintf("Identify risks, limitations, and unresolved questions to make %s plans more robust.", concept),
			keys:    append([]string{"risks", "open questions"}, keyTopics...),
		},
	}

	results := make([]models.Direction, 0, len(plans))
	for i, plan := range plans {
		if i >= 3 && len(context) < 3 {
			break
		}
		d := models.Direction{
			Type:        plan.dirType,
			Title:       plan.title,
			Description: plan.desc,
			Keywords:    uniqueStrings(plan.keys),
			Relevance:   math.Min(1, baseRelevance-0.05*float64(i)),
		}
		results = append(results, d)
	}

	return results
}

func truncate(input string, max int) string {
	if len([]rune(input)) <= max {
		return input
	}
	runes := []rune(input)
	return string(runes[:max])
}
