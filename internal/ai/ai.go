package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arpxspace/smartcommit/internal/config"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// Provider defines the interface for AI providers.
type Provider interface {
	GenerateQuestions(ctx context.Context, diff string, history string) ([]string, error)
	GenerateCommitMessage(ctx context.Context, diff string, history string, answers map[string]string) (string, error)
	AnalyzeHistory(ctx context.Context, diff string, history string) (*HistoryAnalysisResponse, error)
}

// NewClient creates a new AI provider based on the configuration.
func NewClient(cfg *config.Config) (Provider, error) {
	switch cfg.Provider {
	case config.ProviderOpenAI:
		return NewOpenAIClient(cfg.OpenAIAPIKey), nil
	case config.ProviderOllama:
		return NewOllamaClient(cfg.OllamaURL, cfg.OllamaModel), nil
	default:
		// Default to OpenAI if unknown, or error?
		// For backward compatibility, if key is present, assume OpenAI.
		if cfg.OpenAIAPIKey != "" {
			return NewOpenAIClient(cfg.OpenAIAPIKey), nil
		}
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// GenerateSchema creates a JSON schema for a given type T.
// This is used for OpenAI Structured Outputs.
func GenerateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// --- OpenAI Implementation ---

type OpenAIClient struct {
	client *openai.Client
}

func NewOpenAIClient(apiKey string) *OpenAIClient {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIClient{
		client: &client,
	}
}

type QuestionsResponse struct {
	Questions []string `json:"questions" jsonschema_description:"A list of 3 short, specific questions to ask the user to clarify the intent and context of the changes."`
}

// Generate the JSON schema at initialization time
var QuestionsResponseSchema = GenerateSchema[QuestionsResponse]()

func (c *OpenAIClient) GenerateQuestions(ctx context.Context, diff string, history string) ([]string, error) {
	systemPrompt := `You are an expert software developer assisting a user in writing a commit message.
Your goal is to understand the "why" behind the changes.
Analyze the provided git diff and recent project history.
Generate 3 short, specific questions to ask the user to clarify the intent and context of the changes.
The questions should focus on the "why" and "how" if it's not obvious.`

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s", diff, history)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "questions_response",
		Description: openai.String("List of clarifying questions"),
		Schema:      QuestionsResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: openai.ChatModelGPT4o2024_08_06,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to generate questions: %w", err)
	}

	var result QuestionsResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result.Questions, nil
}

type CommitMessageResponse struct {
	Subject string `json:"subject" jsonschema_description:"The commit message subject line, following Conventional Commits specification."`
	Body    string `json:"body" jsonschema_description:"The detailed commit message body explaining the 'what' and 'why'."`
}

// Generate the JSON schema at initialization time
var CommitMessageResponseSchema = GenerateSchema[CommitMessageResponse]()

func (c *OpenAIClient) GenerateCommitMessage(ctx context.Context, diff string, history string, answers map[string]string) (string, error) {
	systemPrompt := `You are an expert software developer.
Generate a commit message following the Conventional Commits specification.
Use the provided diff, recent project history, and user answers to context questions.
The commit message should have a clear subject line and a detailed body explaining the "what" and "why".
Ensure the tone is professional and consistent with the project history.`

	qaPairs := ""
	for q, a := range answers {
		qaPairs += fmt.Sprintf("Q: %s\nA: %s\n", q, a)
	}

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s\n\nUser Context:\n%s", diff, history, qaPairs)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "commit_message_response",
		Description: openai.String("A structured commit message"),
		Schema:      CommitMessageResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: openai.ChatModelGPT4o2024_08_06,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	var result CommitMessageResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return fmt.Sprintf("%s\n\n%s", result.Subject, result.Body), nil
}

type HistoryAnalysisResponse struct {
	IsRelevant bool     `json:"is_relevant" jsonschema_description:"Whether the recent history is relevant to the current changes."`
	KeyContext []string `json:"key_context" jsonschema_description:"A list of key context points from the history that are relevant to the current changes."`
}

// Generate the JSON schema at initialization time
var HistoryAnalysisResponseSchema = GenerateSchema[HistoryAnalysisResponse]()

func (c *OpenAIClient) AnalyzeHistory(ctx context.Context, diff string, history string) (*HistoryAnalysisResponse, error) {
	systemPrompt := `You are an expert software developer.
Analyze the provided git diff and recent project history.
Determine if the recent history is relevant to the current changes (e.g., similar files, related features, bug fixes).
If relevant, extract key context points that should be kept in mind when writing the commit message.
If not relevant, indicate so.`

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s", diff, history)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "history_analysis_response",
		Description: openai.String("Analysis of project history relevance"),
		Schema:      HistoryAnalysisResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: openai.ChatModelGPT4o2024_08_06,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze history: %w", err)
	}

	var result HistoryAnalysisResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// --- Ollama Implementation ---

type OllamaClient struct {
	client *openai.Client
	model  string
}

func NewOllamaClient(baseURL, model string) *OllamaClient {
	// Ensure BaseURL ends with /v1/ for OpenAI compatibility
	// Simple heuristic: if it doesn't contain /v1, append it.
	// This handles the default "http://localhost:11434" -> "http://localhost:11434/v1/"
	if baseURL != "" && baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	if len(baseURL) < 3 || baseURL[len(baseURL)-3:] != "v1/" {
		baseURL += "v1/"
	}

	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey("ollama"), // Required but unused by Ollama
	)

	return &OllamaClient{
		client: &client,
		model:  model,
	}
}

func (c *OllamaClient) GenerateQuestions(ctx context.Context, diff string, history string) ([]string, error) {
	systemPrompt := `You are an expert software developer assisting a user in writing a commit message.
Your goal is to understand the "why" behind the changes.
Analyze the provided git diff and recent project history.

IMPORTANT:
- Your primary focus MUST be on the STAGED CHANGES (the diff).
- The recent project history is provided ONLY as supporting context to understand the project's style and ongoing work.
- Do NOT ask questions about the history unless it directly relates to the current changes.

Generate 3 short, specific questions to ask the user to clarify the intent and context of the changes.

Guidelines:
- Focus on the "why" and "intent", not just the "what".
- Avoid generic questions like "What does this change do?".
- If the changes are self-explanatory, ask for any extra context or side effects.

Examples of GOOD questions:
- "Why was the timeout increased to 5 seconds?"
- "What edge case does this nil check handle?"
- "Is this refactor part of a larger cleanup?"

Examples of BAD questions:
- "Did you update the file?"
- "What is the new value of X?"`

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s", diff, history)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "questions_response",
		Description: openai.String("List of clarifying questions"),
		Schema:      QuestionsResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: c.model,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to generate questions: %w", err)
	}

	var result QuestionsResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result.Questions, nil
}

func (c *OllamaClient) GenerateCommitMessage(ctx context.Context, diff string, history string, answers map[string]string) (string, error) {
	systemPrompt := `You are an expert software developer.
Generate a commit message following the Conventional Commits specification.
Use the provided diff, recent project history, and user answers to context questions.

Rules:
1. The subject line MUST be in the format: <type>(<scope>): <description>
2. Allowed types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert.
3. Keep the subject under 50 characters if possible.
4. The body should explain "what" and "why", not just "how".
5. Use the user's answers to provide specific context.

Template:
<type>(<scope>): <subject>

<body>`

	qaPairs := ""
	for q, a := range answers {
		qaPairs += fmt.Sprintf("Q: %s\nA: %s\n", q, a)
	}

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s\n\nUser Context:\n%s", diff, history, qaPairs)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "commit_message_response",
		Description: openai.String("A structured commit message"),
		Schema:      CommitMessageResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: c.model,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	var result CommitMessageResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return fmt.Sprintf("%s\n\n%s", result.Subject, result.Body), nil
}

func (c *OllamaClient) AnalyzeHistory(ctx context.Context, diff string, history string) (*HistoryAnalysisResponse, error) {
	systemPrompt := `You are an expert software developer.
Analyze the provided git diff and recent project history.
Determine if the recent history is relevant to the current changes (e.g., similar files, related features, bug fixes).
If relevant, extract key context points that should be kept in mind when writing the commit message.
If not relevant, indicate so.`

	userPrompt := fmt.Sprintf("Diff:\n%s\n\nRecent History:\n%s", diff, history)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "history_analysis_response",
		Description: openai.String("Analysis of project history relevance"),
		Schema:      HistoryAnalysisResponseSchema,
		Strict:      openai.Bool(true),
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model: c.model,
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze history: %w", err)
	}

	var result HistoryAnalysisResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}
