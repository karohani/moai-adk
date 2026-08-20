package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// @MX:ANCHOR: [AUTO] openAIStreamTranslator is the SSE event-stream
// reconstruction state machine (AC-PROXY-012a/012b) — OpenAI-shaped
// backend chunk boundaries are NOT 1:1 with Anthropic SSE events, so a
// single mapper function cannot do this; state must be tracked across
// Feed calls.
// @MX:REASON: fan_in >= 3 — TranslateOpenAIStreamToAnthropicSSE (the
// io.Reader wrapper), openai_compatible.go's streaming handler path, and
// codex_group.go's streaming handler path (per REQ-PROXY-018, both
// OpenAI-shaped consumers share this SAME translator) all construct and
// drive one of these per request.

// openAIStreamChunk is the minimal shape this translator needs to read off
// one OpenAI Chat-Completions-shaped streaming chunk.
type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIStreamUsage   `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []openAIStreamToolCall `json:"tool_calls,omitempty"`
}

type openAIStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type openAIStreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// toolCallBlockState tracks ONE tool_calls[].index's translation progress:
// the Anthropic content-block index it was assigned, and whether its
// content_block_start has been emitted yet (it can only be emitted once
// the id+name arrive, which is not necessarily the same chunk that opens
// the fragment sequence — some backends split id/name into their own
// leading chunk with an empty arguments string).
type toolCallBlockState struct {
	blockIndex int
	started    bool
}

// openAIStreamTranslator is a pure Feed(line)->frames state machine. It
// makes no I/O of its own — TranslateOpenAIStreamToAnthropicSSE is the
// thin io.Reader/io.Writer glue around it, kept separate so the state
// machine itself is fully unit-testable without a real network stream
// (translate_stream_test.go feeds it literal lines directly).
type openAIStreamTranslator struct {
	messageID string
	model     string

	started bool // message_start emitted

	textBlockIndex int
	textBlockOpen  bool
	textBlockDone  bool

	toolBlocks     map[int]*toolCallBlockState // openai tool_calls[].index -> state
	toolBlockOrder []int                       // insertion order, for deterministic close order

	nextBlockIndex int

	finishReason string // "" until a non-null finish_reason chunk is seen
	terminating  bool   // finish_reason processed, blocks closed, awaiting final flush
	terminated   bool   // message_delta+message_stop already emitted (idempotency guard)

	completionTokens int
}

// newOpenAIStreamTranslator constructs a translator for one request.
// messageID becomes the Anthropic response's message.id; model becomes
// message_start's message.model.
func newOpenAIStreamTranslator(messageID, model string) *openAIStreamTranslator {
	return &openAIStreamTranslator{
		messageID:  messageID,
		model:      model,
		toolBlocks: make(map[int]*toolCallBlockState),
	}
}

// Feed processes ONE raw line from the backend's SSE stream (expected to
// be either a "data: {...}" JSON chunk, a literal "data: [DONE]" sentinel,
// or a blank/non-data SSE line to ignore) and returns zero or more
// already-SSE-framed (`event: X\ndata: Y\n\n`) byte blocks to relay to the
// client, in order.
func (tr *openAIStreamTranslator) Feed(line []byte) [][]byte {
	trimmed := strings.TrimRight(string(line), "\r\n")
	if !strings.HasPrefix(trimmed, "data:") {
		return nil // blank line, "event:" field, ": comment", etc. — not a data payload
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" {
		return nil
	}

	var frames [][]byte
	frames = append(frames, tr.ensureStarted()...)

	if payload == "[DONE]" {
		frames = append(frames, tr.finalize()...)
		return frames
	}

	var chunk openAIStreamChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		// A malformed chunk is skipped rather than aborting the whole
		// stream — the client still receives a well-formed (if
		// incomplete) Anthropic-shaped stream via the eventual [DONE]/EOF
		// flush, rather than a hard failure mid-relay.
		return frames
	}

	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0] // v1 scope: single-choice (n=1) streams only.
		frames = append(frames, tr.applyDelta(choice.Delta)...)
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			tr.finishReason = *choice.FinishReason
			frames = append(frames, tr.closeBlocks()...)
		}
	}

	if chunk.Usage != nil {
		tr.completionTokens = chunk.Usage.CompletionTokens
		if tr.terminating && !tr.terminated {
			frames = append(frames, tr.emitMessageEnd()...)
		}
	}

	return frames
}

// Close flushes any deferred termination frames — used by the io.Reader
// wrapper when the underlying stream reaches EOF without an explicit
// [DONE] sentinel, so the client-facing stream is never left dangling.
func (tr *openAIStreamTranslator) Close() [][]byte {
	return tr.finalize()
}

func (tr *openAIStreamTranslator) ensureStarted() [][]byte {
	if tr.started {
		return nil
	}
	tr.started = true
	return [][]byte{sseFrame("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            tr.messageID,
			"type":          "message",
			"role":          "assistant",
			"content":       []interface{}{},
			"model":         tr.model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]interface{}{"input_tokens": 0, "output_tokens": 0},
		},
	})}
}

func (tr *openAIStreamTranslator) applyDelta(delta openAIStreamDelta) [][]byte {
	var frames [][]byte

	if delta.Content != "" {
		if !tr.textBlockOpen && !tr.textBlockDone {
			tr.textBlockIndex = tr.nextBlockIndex
			tr.nextBlockIndex++
			tr.textBlockOpen = true
			frames = append(frames, sseFrame("content_block_start", map[string]interface{}{
				"type":          "content_block_start",
				"index":         tr.textBlockIndex,
				"content_block": map[string]interface{}{"type": "text", "text": ""},
			}))
		}
		frames = append(frames, sseFrame("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": tr.textBlockIndex,
			"delta": map[string]interface{}{"type": "text_delta", "text": delta.Content},
		}))
	}

	for _, tc := range delta.ToolCalls {
		state, exists := tr.toolBlocks[tc.Index]
		if !exists {
			state = &toolCallBlockState{blockIndex: tr.nextBlockIndex}
			tr.nextBlockIndex++
			tr.toolBlocks[tc.Index] = state
			tr.toolBlockOrder = append(tr.toolBlockOrder, tc.Index)
		}
		if !state.started && tc.ID != "" && tc.Function.Name != "" {
			state.started = true
			frames = append(frames, sseFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.blockIndex,
				"content_block": map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": map[string]interface{}{},
				},
			}))
		}
		if state.started && tc.Function.Arguments != "" {
			frames = append(frames, sseFrame("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": state.blockIndex,
				"delta": map[string]interface{}{"type": "input_json_delta", "partial_json": tc.Function.Arguments},
			}))
		}
	}

	return frames
}

// closeBlocks closes every open content block (text, then tool blocks in
// arrival order) and marks the translator "terminating" — the final
// message_delta/message_stop pair is DEFERRED until finalize() is called
// (by a usage-only trailing chunk, [DONE], or Close()/EOF), so a
// backend that sends usage in a separate trailing chunk (OpenAI's
// stream_options.include_usage shape) is not shortchanged with a
// zero-usage message_delta.
func (tr *openAIStreamTranslator) closeBlocks() [][]byte {
	if tr.terminating {
		return nil
	}
	tr.terminating = true

	var frames [][]byte
	if tr.textBlockOpen {
		frames = append(frames, sseFrame("content_block_stop", map[string]interface{}{
			"type": "content_block_stop", "index": tr.textBlockIndex,
		}))
		tr.textBlockOpen = false
		tr.textBlockDone = true
	}
	for _, idx := range tr.toolBlockOrder {
		state := tr.toolBlocks[idx]
		if state.started {
			frames = append(frames, sseFrame("content_block_stop", map[string]interface{}{
				"type": "content_block_stop", "index": state.blockIndex,
			}))
		}
	}
	return frames
}

// finalize is the single path that emits message_delta+message_stop,
// invoked from three places (payload=="[DONE]", a post-terminating usage
// chunk, or Close()/EOF) — each guarded by the terminated flag so the pair
// is emitted EXACTLY once regardless of which trigger fires first.
func (tr *openAIStreamTranslator) finalize() [][]byte {
	if tr.terminated {
		return nil
	}
	var frames [][]byte
	if !tr.terminating {
		// [DONE]/EOF arrived with no prior finish_reason chunk (aborted or
		// malformed stream) — force-close whatever is open so the client
		// still receives a well-formed Anthropic-shaped stream.
		frames = append(frames, tr.closeBlocks()...)
	}
	frames = append(frames, tr.emitMessageEnd()...)
	return frames
}

func (tr *openAIStreamTranslator) emitMessageEnd() [][]byte {
	if tr.terminated {
		return nil
	}
	tr.terminated = true

	stopReason := finishReasonToStopReason[tr.finishReason]
	if stopReason == "" && tr.finishReason != "" {
		stopReason = tr.finishReason
	}
	if stopReason == "" {
		stopReason = "end_turn"
	}

	return [][]byte{
		sseFrame("message_delta", map[string]interface{}{
			"type":  "message_delta",
			"delta": map[string]interface{}{"stop_reason": stopReason, "stop_sequence": nil},
			"usage": map[string]interface{}{"output_tokens": tr.completionTokens},
		}),
		sseFrame("message_stop", map[string]interface{}{"type": "message_stop"}),
	}
}

// sseFrame marshals payload to JSON and formats it as one
// `event: <event>\ndata: <json>\n\n` SSE frame.
func sseFrame(event string, payload interface{}) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		// Marshaling a map[string]interface{} literal this function
		// itself constructs cannot fail in practice; this is a defensive
		// fallback that keeps the stream well-formed rather than panicking.
		data = []byte(`{}`)
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
}

// TranslateOpenAIStreamToAnthropicSSE wraps an OpenAI-shaped backend's raw
// SSE response body (r) in an io.ReadCloser that emits the translated
// Anthropic-shaped SSE stream, via a background goroutine driving an
// openAIStreamTranslator over r's lines. messageID/model seed the
// translator's message_start payload.
func TranslateOpenAIStreamToAnthropicSSE(r io.Reader, messageID, model string) io.ReadCloser {
	pr, pw := io.Pipe()
	tr := newOpenAIStreamTranslator(messageID, model)

	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var writeErr error
		for scanner.Scan() {
			for _, frame := range tr.Feed(scanner.Bytes()) {
				if _, err := pw.Write(frame); err != nil {
					writeErr = err
					break
				}
			}
			if writeErr != nil {
				break
			}
		}
		if writeErr == nil {
			for _, frame := range tr.Close() {
				if _, err := pw.Write(frame); err != nil {
					writeErr = err
					break
				}
			}
		}
		if writeErr == nil {
			writeErr = scanner.Err()
		}
		_ = pw.CloseWithError(writeErr)
	}()

	return pr
}
