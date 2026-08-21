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
}

// ToOpenAIChatRequest translates an Anthropic /v1/messages request body
// into an OpenAI Chat Completions request body.
//
// Field mapping (progress.md §E.2 M3 carries the full table):
//   - model: KEPT in the body — unlike Bedrock's InvokeModel (which takes
//     the model out-of-band via a URL parameter), OpenAI Chat Completions
//     requires "model" as a body field.
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
func ToOpenAIChatRequest(anthropicBody []byte) ([]byte, error) {
	var req struct {
		Model         string          `json:"model"`
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
		"model": req.Model,
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
	if req.Stream {
		out["stream"] = true
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
		case "tool_result":
			toolResultMessages = append(toolResultMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": b.ToolUseID,
				"content":      toolResultContentToString(b.Content),
			})
		}
	}

	var result []interface{}
	if len(textParts) > 0 || len(toolCalls) > 0 {
		msg := map[string]interface{}{"role": m.Role}
		if len(textParts) > 0 {
			msg["content"] = strings.Join(textParts, "")
		} else {
			msg["content"] = nil
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		result = append(result, msg)
	}
	result = append(result, toolResultMessages...)
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
func FromOpenAIChatResponse(openaiBody []byte) ([]byte, error) {
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

	var content []interface{}
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

	stopReason := finishReasonToStopReason[choice.FinishReason]
	if stopReason == "" {
		stopReason = choice.FinishReason
	}

	out := map[string]interface{}{
		"id":          resp.ID,
		"type":        "message",
		"role":        "assistant",
		"content":     content,
		"stop_reason": stopReason,
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
