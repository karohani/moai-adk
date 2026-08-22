package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
)

// @MX:ANCHOR: [AUTO] ToOpenAIChatRequest / FromOpenAIChatResponse are the
// single shared OpenAI<->Anthropic translation entry points — REQ-PROXY-018
// requires every OpenAI-shaped group type (codex, openai-compatible, and a
// future copilot) to call these SAME functions, never a per-type parallel
// translation implementation.
// @MX:REASON: fan_in >= 3 — internal/proxy/openai_compatible.go,
// internal/proxy/codex_group.go, and (per REQ-PROXY-018, when copilot is
// eventually revived) a future copilot adapter all call these functions;
// AC-PROXY-011 fails if any consumer bypasses them with its own logic.

// finishReasonToStopReason maps OpenAI's `finish_reason` to Anthropic's
// `stop_reason`. Only the values a Chat-Completions-shaped backend actually
// emits in this proxy's v1 scope are mapped; an unrecognized value passes
// through unchanged rather than being silently coerced to a guessed value.
var finishReasonToStopReason = map[string]string{
	"stop":       "end_turn",
	"length":     "max_tokens",
	"tool_calls": "tool_use",
	// OpenAI emits content_filter when its moderation layer stops
	// generation. Anthropic's nearest member is refusal; passing
	// "content_filter" through verbatim (the pre-fix behavior) put a
	// non-member of Anthropic's stop_reason enum on the wire, which strict
	// clients reject.
	"content_filter": "refusal",
	"function_call":  "tool_use", // deprecated OpenAI alias for tool_calls
	// Emitted by the streaming translator when the backend stream ended
	// without ever reporting a finish reason (see CloseTruncated).
	truncatedFinishReason: "refusal",
}

// coerceStopReason maps an OpenAI finish_reason onto a VALID Anthropic
// stop_reason. An unrecognized value falls back to "end_turn" rather than
// passing through: emitting a value outside Anthropic's enum breaks
// schema-validating clients, and there is no safe way to invent a new
// enum member on their behalf.
func coerceStopReason(finishReason string) string {
	if mapped, ok := finishReasonToStopReason[finishReason]; ok {
		return mapped
	}
	return "end_turn"
}

// ToOpenAIChatRequest translates an Anthropic /v1/messages request body
// into an OpenAI Chat Completions request body.
//
// Field mapping (progress.md §E.2 M3 carries the full table):
//   - model: KEPT in the body — unlike Bedrock's InvokeModel (which takes
//     the model out-of-band via a URL parameter), OpenAI Chat Completions
//     requires "model" as a body field. The caller-supplied model param is
//     used verbatim — NOT the request body's own "model" field, which for
//     a catalog-resolved deployment still carries the client-facing
//     "<group>/<model>" alias rather than the backend's real model id.
//   - system: Anthropic's top-level `system` (a string OR a content-block
//     array — both forms occur in the wild; Claude Code may send either)
//     becomes a LEADING message with role "system". Omitted entirely if
//     the request carries no system field.
//   - stop_sequences -> stop (renamed, structure otherwise unchanged).
//   - top_k: DROPPED. OpenAI Chat Completions has no equivalent parameter.
//   - max_tokens / temperature / top_p: passthrough, unchanged names.
//   - tools: Anthropic {name, description, input_schema} -> OpenAI
//     {type: "function", function: {name, description, parameters}}.
//   - messages: Anthropic content-block messages (text / tool_use /
//     tool_result) are flattened into OpenAI's {role, content, tool_calls}
//     / {role: "tool", tool_call_id, content} shapes.
func ToOpenAIChatRequest(anthropicBody []byte, model string) ([]byte, error) {
	var req struct {
		System        json.RawMessage `json:"system,omitempty"`
		Messages      []anthMessage   `json:"messages"`
		MaxTokens     json.Number     `json:"max_tokens,omitempty"`
		Temperature   json.Number     `json:"temperature,omitempty"`
		TopP          json.Number     `json:"top_p,omitempty"`
		StopSequences []string        `json:"stop_sequences,omitempty"`
		Tools         []anthTool      `json:"tools,omitempty"`
		ToolChoice    json.RawMessage `json:"tool_choice,omitempty"`
		Stream        bool            `json:"stream,omitempty"`
	}
	if err := json.Unmarshal(anthropicBody, &req); err != nil {
		return nil, fmt.Errorf("proxy: parse anthropic request body: %w", err)
	}

	out := map[string]interface{}{
		"model": model,
	}

	var messages []interface{}
	if len(req.System) > 0 {
		systemText, err := extractSystemText(req.System)
		if err != nil {
			return nil, err
		}
		if systemText != "" {
			messages = append(messages, map[string]interface{}{"role": "system", "content": systemText})
		}
	}
	for _, m := range req.Messages {
		translated, err := translateAnthropicMessageToOpenAI(m)
		if err != nil {
			return nil, err
		}
		messages = append(messages, translated...)
	}
	out["messages"] = messages

	if req.MaxTokens.String() != "" {
		out["max_tokens"] = jsonNumberToInterface(req.MaxTokens)
	}
	if req.Temperature.String() != "" {
		out["temperature"] = jsonNumberToInterface(req.Temperature)
	}
	if req.TopP.String() != "" {
		out["top_p"] = jsonNumberToInterface(req.TopP)
	}
	if len(req.StopSequences) > 0 {
		out["stop"] = req.StopSequences
	}
	// top_k intentionally dropped — no OpenAI Chat Completions equivalent.

	if len(req.Tools) > 0 {
		var tools []interface{}
		for _, t := range req.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			})
		}
		out["tools"] = tools
	}
	if choice, ok := translateToolChoice(req.ToolChoice); ok {
		out["tool_choice"] = choice
	}
	if disableParallel(req.ToolChoice) {
		// Anthropic carries this INSIDE tool_choice; OpenAI has it as a
		// sibling field. Dropping it let the backend fan out tool calls the
		// caller explicitly forbade.
		out["parallel_tool_calls"] = false
	}
	if req.Stream {
		out["stream"] = true
		// stream_options.include_usage is REQUIRED for a spec-conforming
		// OpenAI backend to emit the trailing usage chunk. Without it the
		// streaming translator's deferred-finalize path (which exists
		// precisely to wait for that chunk) can never fire, and every
		// streaming response reports zero tokens.
		out["stream_options"] = map[string]interface{}{"include_usage": true}
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("proxy: marshal openai chat request: %w", err)
	}
	return result, nil
}

// anthMessage is the minimal shape ToOpenAIChatRequest needs to read an
// Anthropic messages[] entry. Content may be a bare string OR an array of
// content blocks (text / tool_use / tool_result) — json.RawMessage defers
// the decision to translateAnthropicMessageToOpenAI.
type anthMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// translateToolChoice maps Anthropic's tool_choice onto OpenAI's. The two
// vocabularies correspond directly, so silently dropping the field (the
// pre-fix behavior) discarded a load-bearing constraint: a caller demanding
// a specific tool would receive prose instead, with no signal that its
// instruction had been ignored.
//
//	Anthropic {"type":"auto"}                  -> "auto"
//	Anthropic {"type":"any"}                   -> "required"
//	Anthropic {"type":"none"}                  -> "none"
//	Anthropic {"type":"tool","name":"x"}       -> {"type":"function","function":{"name":"x"}}
//
// ok is false when the field is absent or unparseable — an unrecognized
// shape is left off the outbound request rather than guessed at, matching
// the top_k drop's "no equivalent, so omit" convention.
// disableParallel reads Anthropic's tool_choice.disable_parallel_tool_use,
// which OpenAI expresses as the top-level parallel_tool_calls field.
func disableParallel(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var tc struct {
		DisableParallelToolUse bool `json:"disable_parallel_tool_use"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return false
	}
	return tc.DisableParallelToolUse
}

func translateToolChoice(raw json.RawMessage) (interface{}, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, false
	}
	switch tc.Type {
	case "auto":
		return "auto", true
	case "any":
		return "required", true
	case "none":
		return "none", true
	case "tool":
		if tc.Name == "" {
			return nil, false
		}
		return map[string]interface{}{
			"type":     "function",
			"function": map[string]interface{}{"name": tc.Name},
		}, true
	default:
		return nil, false
	}
}

// extractSystemText normalizes Anthropic's `system` field, which occurs in
// the wild as EITHER a bare string OR an array of content blocks
// ([{"type":"text","text":"..."}]) — real Anthropic clients (Claude Code
// included) may send either shape. Block-array text is joined with "\n".
func extractSystemText(raw json.RawMessage) (string, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("proxy: system field is neither a string nor a content-block array: %w", err)
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "text" || b.Type == "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

// translateAnthropicMessageToOpenAI translates one Anthropic messages[]
// entry into ZERO OR MORE OpenAI messages[] entries. A single Anthropic
// message can expand to more than one OpenAI message: an assistant message
// mixing text and tool_use content stays ONE OpenAI message (content +
// tool_calls together), but a user message carrying tool_result blocks
// expands to one OpenAI "tool" role message PER tool_result block.
func translateAnthropicMessageToOpenAI(m anthMessage) ([]interface{}, error) {
	// Bare-string content is the common case for simple text turns.
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		return []interface{}{map[string]interface{}{"role": m.Role, "content": asString}}, nil
	}

	var blocks []anthContentBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil, fmt.Errorf("proxy: message content is neither a string nor a content-block array: %w", err)
	}

	var textParts []string
	var imageParts []interface{}
	var toolCalls []interface{}
	var toolResultMessages []interface{}

	for _, b := range blocks {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "tool_use":
			args, err := json.Marshal(b.Input)
			if err != nil {
				return nil, fmt.Errorf("proxy: marshal tool_use input for %q: %w", b.ID, err)
			}
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   b.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      b.Name,
					"arguments": string(args),
				},
			})
		case "image":
			// OpenAI Chat Completions DOES represent images, as an
			// image_url content part carrying a data: URI. Translating is
			// strictly better than either dropping the image (the original
			// defect) or rejecting the request: a multimodal turn keeps
			// working against a backend that supports it.
			part, err := anthropicImageToOpenAIPart(b)
			if err != nil {
				return nil, err
			}
			imageParts = append(imageParts, part)
		case "tool_result":
			toolResultMessages = append(toolResultMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": b.ToolUseID,
				"content":      toolResultContentToString(b.Content),
			})
		default:
			if assistantInternalBlockTypes[b.Type] {
				// Assistant-internal scaffolding from an EARLIER turn
				// (reasoning traces, server-side tool bookkeeping). OpenAI
				// has no representation, and omitting it loses no user
				// intent — the assistant's own `text` answer from that turn
				// is translated normally alongside it. Erroring here would
				// break every multi-turn session that has extended thinking
				// enabled, because Anthropic requires those blocks to be
				// echoed back in the message history.
				continue
			}
			// Everything else carries USER intent the model is being asked
			// to reason about (image, document, ...). Dropping it ships an
			// incomplete prompt to the backend, which then answers
			// confidently about content it never saw — a failure invisible
			// to the caller and indistinguishable from a real answer.
			// Failing loudly is the only safe option for a translating
			// proxy.
			return nil, fmt.Errorf(
				"proxy: anthropic content block type %q carries user content with no OpenAI Chat Completions equivalent and cannot be translated; route this request to an anthropic-native group (bedrock/litellm) instead",
				b.Type)
		}
	}

	var result []interface{}
	// OpenAI requires a role:"tool" message to IMMEDIATELY follow the
	// assistant message carrying the matching tool_calls. Emitting the
	// turn's own text message first would interpose a user message between
	// the two and violate that sequencing, so tool results lead.
	result = append(result, toolResultMessages...)
	if len(textParts) > 0 || len(toolCalls) > 0 || len(imageParts) > 0 {
		msg := map[string]interface{}{"role": m.Role}
		switch {
		case len(imageParts) > 0:
			// OpenAI's multimodal form: content becomes an array of parts.
			// Text leads so the image follows the instruction that refers
			// to it, matching how Anthropic clients order the blocks.
			parts := make([]interface{}, 0, len(imageParts)+1)
			if len(textParts) > 0 {
				parts = append(parts, map[string]interface{}{
					"type": "text", "text": strings.Join(textParts, ""),
				})
			}
			parts = append(parts, imageParts...)
			msg["content"] = parts
		case len(textParts) > 0:
			msg["content"] = strings.Join(textParts, "")
		default:
			msg["content"] = nil
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		result = append(result, msg)
	}
	return result, nil
}

type anthContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   json.RawMessage        `json:"content,omitempty"`
	Source    anthImageSource        `json:"source,omitempty"`
}

// anthImageSource is Anthropic's image payload: either inline base64 with a
// media type, or a URL.
type anthImageSource struct {
	Type      string `json:"type,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// assistantInternalBlockTypes names content blocks that carry the
// ASSISTANT's own prior scaffolding rather than user intent. OpenAI has no
// representation for them, but omitting them loses nothing the model needs:
// the assistant's `text` from that same turn is translated alongside.
//
// They must be skipped rather than rejected because Anthropic REQUIRES
// thinking blocks to be echoed back in the message history once extended
// thinking is enabled — erroring would break every multi-turn session
// against an OpenAI-shaped backend from its second turn onward.
var assistantInternalBlockTypes = map[string]bool{
	"thinking":               true,
	"redacted_thinking":      true,
	"server_tool_use":        true,
	"web_search_tool_result": true,
}

// anthropicImageToOpenAIPart converts an Anthropic image block into an
// OpenAI image_url content part. Anthropic carries the bytes inline
// (base64 + media_type) or by URL; OpenAI takes a single url field, so a
// base64 source becomes a data: URI.
func anthropicImageToOpenAIPart(b anthContentBlock) (interface{}, error) {
	switch b.Source.Type {
	case "base64":
		if b.Source.MediaType == "" || b.Source.Data == "" {
			return nil, fmt.Errorf("proxy: image block has an incomplete base64 source (media_type or data missing)")
		}
		return map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": "data:" + b.Source.MediaType + ";base64," + b.Source.Data,
			},
		}, nil
	case "url":
		if b.Source.URL == "" {
			return nil, fmt.Errorf("proxy: image block has a url source with no url")
		}
		return map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]interface{}{"url": b.Source.URL},
		}, nil
	default:
		return nil, fmt.Errorf("proxy: image block source type %q is not supported", b.Source.Type)
	}
}

// toolResultContentToString normalizes a tool_result block's content
// (which may be a bare string or a content-block array, mirroring the
// system-field ambiguity) into a single string for OpenAI's "tool" role
// message, whose content is always a string.
func toolResultContentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		parts := make([]string, 0, len(blocks))
		for _, b := range blocks {
			parts = append(parts, b.Text)
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

func jsonNumberToInterface(n json.Number) interface{} {
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return f
	}
	return n.String()
}

// FromOpenAIChatResponse translates an OpenAI Chat Completions response
// body (non-streaming) into an Anthropic /v1/messages response body.
func FromOpenAIChatResponse(openaiBody []byte, model string) ([]byte, error) {
	var resp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Role      string  `json:"role"`
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(openaiBody, &resp); err != nil {
		return nil, fmt.Errorf("proxy: parse openai chat response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("proxy: openai chat response carries no choices")
	}
	choice := resp.Choices[0]

	// Non-nil so an empty result marshals to `[]`, not `null` — Anthropic's
	// schema types content as an array, and a null breaks clients that
	// iterate it without a guard.
	content := []interface{}{}
	if choice.Message.Content != nil && *choice.Message.Content != "" {
		content = append(content, map[string]interface{}{"type": "text", "text": *choice.Message.Content})
	}
	for _, tc := range choice.Message.ToolCalls {
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("proxy: parse tool_call %q arguments as JSON: %w", tc.ID, err)
		}
		content = append(content, map[string]interface{}{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}

	out := map[string]interface{}{
		"id":            resp.ID,
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"model":         model,
		"stop_reason":   coerceStopReason(choice.FinishReason),
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		},
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("proxy: marshal anthropic response: %w", err)
	}
	return result, nil
}
