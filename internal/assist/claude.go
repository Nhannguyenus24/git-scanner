package assist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// Model is the Claude model the assistant uses.
const Model = "claude-opus-5-5"

// Claude is the Translator backed by the Claude API.
type Claude struct {
	client anthropic.Client
}

// NewClaude builds the translator. The call is bounded: the dashboard waits on
// it, and a filter that takes longer than this is worth less than an error.
// Extra options come after the defaults, so a test can point it elsewhere.
func NewClaude(apiKey string, opts ...option.RequestOption) *Claude {
	base := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(20 * time.Second),
		option.WithMaxRetries(1),
	}
	return &Claude{client: anthropic.NewClient(append(base, opts...)...)}
}

const system = `You turn a question about leaked credentials into a filter for a findings list.
Answer only with the JSON object the schema describes. Leave a field as "" when the question does not ask for it; never guess a value the question does not imply.

Fields:
- severity: one of critical, high, medium, low, info.
- provider: one of the provider slugs listed in the user message, or "unmapped" for credentials no provider rule claimed. Map product names to slugs (for example "Amazon" or "AWS keys" -> the aws slug).
- status: open (not yet dealt with), rotated, false_positive, ignored.
- exposure: head (still in the current code / default branch), history (removed but still in git history), live (confirmed still working by the liveness check).
- sort: recent (newest detections first), oldest (oldest commit first), severity.
- q: free-text search over file path, rule, provider and repository — use it for repository, owner, organisation or file names. Keep it short.
- explanation: one short sentence, in the language of the question, saying how you read it.`

func schema() map[string]any {
	str := func(enum ...string) map[string]any {
		if len(enum) == 0 {
			return map[string]any{"type": "string"}
		}
		return map[string]any{"type": "string", "enum": append([]string{""}, enum...)}
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"severity":    str(Severities...),
			"provider":    str(),
			"status":      str(Statuses...),
			"exposure":    str(Exposures...),
			"sort":        str(Sorts...),
			"q":           str(),
			"explanation": str(),
		},
		"required":             []string{"severity", "provider", "status", "exposure", "sort", "q", "explanation"},
		"additionalProperties": false,
	}
}

// Translate asks Claude to read the question. The answer is constrained to the
// schema by structured outputs and then sanitised again here, because the
// schema cannot know which providers this server has.
func (c *Claude) Translate(ctx context.Context, question string, providers []string) (Filter, error) {
	q, err := CheckQuestion(question)
	if err != nil {
		return Filter{}, err
	}
	user := fmt.Sprintf("Provider slugs: %s\n\nQuestion: %s", strings.Join(providers, ", "), q)

	msg, err := c.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     Model,
		MaxTokens: 4096,
		// A refusal is re-served by Anthropic's recommended fallback model
		// inside the same call rather than returned as a dead end.
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(user)),
		},
		OutputConfig: anthropic.BetaOutputConfigParam{
			// Parsing a sentence into seven fields is not hard reasoning.
			Effort: anthropic.BetaOutputConfigEffortLow,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: schema()},
		},
	})
	if err != nil {
		return Filter{}, fmt.Errorf("assist: claude: %w", err)
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return Filter{}, ErrRefused
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if t, ok := b.AsAny().(anthropic.BetaTextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	var f Filter
	if err := json.Unmarshal([]byte(text.String()), &f); err != nil {
		return Filter{}, fmt.Errorf("assist: unreadable answer (stop reason %s): %w", msg.StopReason, err)
	}
	return Sanitize(f, providers), nil
}
