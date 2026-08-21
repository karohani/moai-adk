package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// sseEvents splits a raw `event: X\ndata: Y\n\n`-framed byte stream into a
// slice of (event, data) pairs, in order, for assertions.
func sseEvents(t *testing.T, raw []byte) []struct{ Event, Data string } {
	t.Helper()
	var out []struct{ Event, Data string }
	blocks := strings.Split(string(raw), "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var ev, data string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				ev = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data = strings.TrimPrefix(line, "data: ")
			}
		}
		out = append(out, struct{ Event, Data string }{ev, data})
	}
	return out
}

// feedLines runs a sequence of raw OpenAI SSE "data: {...}" lines (plus a
// final "data: [DONE]") through a fresh openAIStreamTranslator and
// concatenates every emitted Anthropic-shaped SSE frame.
func feedLines(t *testing.T, lines ...string) []byte {
	t.Helper()
	tr := newOpenAIStreamTranslator("msg_test", "gpt-oss-120b")
	var buf bytes.Buffer
	for _, line := range lines {
		frames := tr.Feed([]byte(line))
		for _, f := range frames {
			buf.Write(f)
		}
	}
	return buf.Bytes()
}

// TestOpenAIStreamTranslator_TextOnly_EventOrderAndAssembly is the direct
// unit-level analogue of AC-PROXY-012a: multiple OpenAI delta chunks ->
// Anthropic start/body/end event structure, assembled text equals the
// concatenation of backend deltas.
func TestOpenAIStreamTranslator_TextOnly_EventOrderAndAssembly(t *testing.T) {
	raw := feedLines(t,
		`data: {"choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":", world"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
		`data: [DONE]`,
	)
	events := sseEvents(t, raw)

	wantOrder := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	if len(events) != len(wantOrder) {
		t.Fatalf("event count = %d, want %d; events=%+v", len(events), len(wantOrder), events)
	}
	for i, want := range wantOrder {
		if events[i].Event != want {
			t.Errorf("event[%d] = %q, want %q", i, events[i].Event, want)
		}
	}

	// Assemble the text deltas and compare to the backend's concatenation.
	assembled := ""
	for _, e := range events {
		if e.Event != "content_block_delta" {
			continue
		}
		var payload struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(e.Data), &payload); err != nil {
			t.Fatalf("unmarshal content_block_delta: %v", err)
		}
		if payload.Delta.Type != "text_delta" {
			t.Errorf("delta.type = %q, want text_delta", payload.Delta.Type)
		}
		assembled += payload.Delta.Text
	}
	if assembled != "Hello, world" {
		t.Errorf("assembled text = %q, want %q", assembled, "Hello, world")
	}

	// message_delta must carry the mapped stop_reason and final usage.
	var msgDelta struct {
		Delta struct {
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(events[5].Data), &msgDelta); err != nil {
		t.Fatalf("unmarshal message_delta: %v", err)
	}
	if msgDelta.Delta.StopReason != "end_turn" {
		t.Errorf("message_delta.delta.stop_reason = %q, want end_turn", msgDelta.Delta.StopReason)
	}
	if msgDelta.Usage.OutputTokens != 2 {
		t.Errorf("message_delta.usage.output_tokens = %d, want 2", msgDelta.Usage.OutputTokens)
	}
}

// TestOpenAIStreamTranslator_ToolCallFragmentReassembly is the direct
// unit-level analogue of AC-PROXY-012b: OpenAI tool_calls fragmented
// across many chunks reassemble into ONE parseable structured input.
func TestOpenAIStreamTranslator_ToolCallFragmentReassembly(t *testing.T) {
	raw := feedLines(t,
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"NYC\"}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
		`data: [DONE]`,
	)
	events := sseEvents(t, raw)

	var startEvent, stopEvent = -1, -1
	var deltas []string
	for i, e := range events {
		switch e.Event {
		case "content_block_start":
			var payload struct {
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
			}
			_ = json.Unmarshal([]byte(e.Data), &payload)
			if payload.ContentBlock.Type == "tool_use" {
				startEvent = i
				if payload.ContentBlock.ID != "call_1" || payload.ContentBlock.Name != "get_weather" {
					t.Errorf("content_block_start tool_use = %+v", payload.ContentBlock)
				}
			}
		case "content_block_delta":
			var payload struct {
				Delta struct {
					Type        string `json:"type"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			_ = json.Unmarshal([]byte(e.Data), &payload)
			if payload.Delta.Type != "input_json_delta" {
				t.Errorf("delta.type = %q, want input_json_delta", payload.Delta.Type)
			}
			deltas = append(deltas, payload.Delta.PartialJSON)
		case "content_block_stop":
			stopEvent = i
		}
	}
	if startEvent == -1 {
		t.Fatal("no content_block_start for the tool_use block found")
	}
	if stopEvent == -1 || stopEvent < startEvent {
		t.Fatal("no content_block_stop after content_block_start for the tool_use block")
	}

	reassembled := strings.Join(deltas, "")
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(reassembled), &parsed); err != nil {
		t.Fatalf("reassembled tool-call arguments %q are not valid JSON: %v", reassembled, err)
	}
	if parsed["location"] != "NYC" {
		t.Errorf("reassembled arguments = %v, want location=NYC", parsed)
	}

	// message_delta must map finish_reason "tool_calls" -> stop_reason "tool_use".
	found := false
	for _, e := range events {
		if e.Event != "message_delta" {
			continue
		}
		found = true
		var md struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		_ = json.Unmarshal([]byte(e.Data), &md)
		if md.Delta.StopReason != "tool_use" {
			t.Errorf("message_delta.delta.stop_reason = %q, want tool_use", md.Delta.StopReason)
		}
	}
	if !found {
		t.Error("no message_delta event emitted")
	}
}

// TestOpenAIStreamTranslator_TextThenToolCall_SeparateBlocks verifies a
// text block followed by a tool call produces TWO distinct content blocks
// at increasing indices, each independently opened and closed.
func TestOpenAIStreamTranslator_TextThenToolCall_SeparateBlocks(t *testing.T) {
	raw := feedLines(t,
		`data: {"choices":[{"index":0,"delta":{"content":"Let me check."},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	)
	events := sseEvents(t, raw)

	var blockIndices []int
	for _, e := range events {
		if e.Event != "content_block_start" {
			continue
		}
		var payload struct {
			Index int `json:"index"`
		}
		_ = json.Unmarshal([]byte(e.Data), &payload)
		blockIndices = append(blockIndices, payload.Index)
	}
	if len(blockIndices) != 2 || blockIndices[0] != 0 || blockIndices[1] != 1 {
		t.Errorf("content_block_start indices = %v, want [0 1]", blockIndices)
	}
}

// TestOpenAIStreamTranslator_UsageOnlyTrailingChunk verifies the
// finish_reason chunk defers message_delta/message_stop until a
// SEPARATE later usage-only chunk arrives (OpenAI's stream_options
// include_usage shape: choices=[] with usage populated), rather than
// emitting a zero-usage message_delta prematurely.
func TestOpenAIStreamTranslator_UsageOnlyTrailingChunk(t *testing.T) {
	raw := feedLines(t,
		`data: {"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":1}}`,
		`data: [DONE]`,
	)
	events := sseEvents(t, raw)

	var msgDeltaCount int
	for _, e := range events {
		if e.Event != "message_delta" {
			continue
		}
		msgDeltaCount++
		var md struct {
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		_ = json.Unmarshal([]byte(e.Data), &md)
		if md.Usage.OutputTokens != 1 {
			t.Errorf("message_delta.usage.output_tokens = %d, want 1 (from the trailing usage-only chunk)", md.Usage.OutputTokens)
		}
	}
	if msgDeltaCount != 1 {
		t.Errorf("message_delta emitted %d times, want exactly 1 (deferred until the usage-only chunk)", msgDeltaCount)
	}
}

// TestOpenAIStreamTranslator_DoneWithoutFinishReasonForceCloses verifies a
// malformed/aborted stream ([DONE] with no prior finish_reason) still
// force-closes any open block and emits a well-formed message_stop, so the
// Anthropic-shaped stream the client receives is never left dangling.
func TestOpenAIStreamTranslator_DoneWithoutFinishReasonForceCloses(t *testing.T) {
	raw := feedLines(t,
		`data: {"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`,
		`data: [DONE]`,
	)
	events := sseEvents(t, raw)

	last := events[len(events)-1]
	if last.Event != "message_stop" {
		t.Fatalf("last event = %q, want message_stop", last.Event)
	}
	var sawStop bool
	for _, e := range events {
		if e.Event == "content_block_stop" {
			sawStop = true
		}
	}
	if !sawStop {
		t.Error("expected a content_block_stop for the still-open text block before message_stop")
	}
}

// TestOpenAIStreamTranslator_ReaderWrapper exercises the io.Reader-facing
// wrapper (TranslateOpenAIStreamToAnthropicSSE) that glues the pure Feed
// state machine to an actual backend SSE stream.
func TestOpenAIStreamTranslator_ReaderWrapper(t *testing.T) {
	backend := `data: {"choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}

data: [DONE]

`
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(backend), "msg_x", "m")
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	events := sseEvents(t, got)
	if len(events) == 0 {
		t.Fatal("expected at least one translated SSE event")
	}
	if events[0].Event != "message_start" {
		t.Errorf("first event = %q, want message_start", events[0].Event)
	}
	if events[len(events)-1].Event != "message_stop" {
		t.Errorf("last event = %q, want message_stop", events[len(events)-1].Event)
	}
}
