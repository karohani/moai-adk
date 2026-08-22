package proxy

import (
	"encoding/json"
	"testing"
)

// TestToOpenAIChatRequest_BasicFieldMapping verifies the field-by-field
// mapping table (progress.md §E.2 M3): model kept (unlike bedrock — OpenAI
// Chat Completions requires model IN the body), system becomes a leading
// system-role message, stop_sequences -> stop, top_k dropped (no OpenAI
// equivalent), everything else passed through under its OpenAI name.
func TestToOpenAIChatRequest_BasicFieldMapping(t *testing.T) {
	in := []byte(`{
		"model": "gpt-oss-120b",
		"system": "be terse",
		"messages": [{"role":"user","content":"hi"}],
		"max_tokens": 512,
		"temperature": 0.5,
		"top_p": 0.9,
		"top_k": 40,
		"stop_sequences": ["END", "STOP"]
	}`)

	out, err := ToOpenAIChatRequest(in, "gpt-oss-120b")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if got["model"] != "gpt-oss-120b" {
		t.Errorf("model = %v, want gpt-oss-120b (OpenAI keeps model IN the body)", got["model"])
	}
	if _, present := got["top_k"]; present {
		t.Error("top_k must be dropped — no OpenAI Chat Completions equivalent")
	}
	if _, present := got["system"]; present {
		t.Error("system must not survive as a top-level field — it becomes a system-role message")
	}
	stop, ok := got["stop"].([]interface{})
	if !ok || len(stop) != 2 || stop[0] != "END" || stop[1] != "STOP" {
		t.Errorf("stop = %v, want [END STOP] (translated from stop_sequences)", got["stop"])
	}
	if got["max_tokens"] != float64(512) {
		t.Errorf("max_tokens = %v, want 512", got["max_tokens"])
	}
	if got["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", got["temperature"])
	}

	msgs, ok := got["messages"].([]interface{})
	if !ok || len(msgs) != 2 {
		t.Fatalf("messages = %v, want 2 entries (system + user)", got["messages"])
	}
	first := msgs[0].(map[string]interface{})
	if first["role"] != "system" || first["content"] != "be terse" {
		t.Errorf("first message = %v, want the system-role message leading", first)
	}
	second := msgs[1].(map[string]interface{})
	if second["role"] != "user" || second["content"] != "hi" {
		t.Errorf("second message = %v, want the original user message", second)
	}
}

// TestToOpenAIChatRequest_SystemAsContentBlockArray verifies the system
// field is also accepted in its content-block-array form (real Anthropic
// clients, including Claude Code, may send `system` as
// [{"type":"text","text":"..."}] rather than a bare string) — both shapes
// must translate to the same system-role message content.
func TestToOpenAIChatRequest_SystemAsContentBlockArray(t *testing.T) {
	in := []byte(`{
		"model": "m",
		"system": [{"type":"text","text":"be terse"},{"type":"text","text":"and polite"}],
		"messages": [{"role":"user","content":"hi"}]
	}`)

	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]interface{})
	first := msgs[0].(map[string]interface{})
	if first["role"] != "system" {
		t.Fatalf("first message role = %v, want system", first["role"])
	}
	want := "be terse\nand polite"
	if first["content"] != want {
		t.Errorf("system content = %q, want %q (joined content-block text)", first["content"], want)
	}
}

// TestToOpenAIChatRequest_NoSystemFieldOmitsSystemMessage verifies a
// request with no system field produces no leading system message.
func TestToOpenAIChatRequest_NoSystemFieldOmitsSystemMessage(t *testing.T) {
	in := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]interface{})
	if len(msgs) != 1 {
		t.Fatalf("messages = %v, want exactly 1 (no system message)", msgs)
	}
}

// TestToOpenAIChatRequest_ToolsTranslation verifies Anthropic tool
// definitions (name/description/input_schema) translate to OpenAI's
// function-tool wrapper shape.
func TestToOpenAIChatRequest_ToolsTranslation(t *testing.T) {
	in := []byte(`{
		"model": "m",
		"messages": [],
		"tools": [{"name":"get_weather","description":"Get weather","input_schema":{"type":"object","properties":{"location":{"type":"string"}}}}]
	}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	tools, ok := got["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %v, want 1 entry", got["tools"])
	}
	tool := tools[0].(map[string]interface{})
	if tool["type"] != "function" {
		t.Errorf("tool type = %v, want function", tool["type"])
	}
	fn := tool["function"].(map[string]interface{})
	if fn["name"] != "get_weather" {
		t.Errorf("function name = %v, want get_weather", fn["name"])
	}
	if fn["description"] != "Get weather" {
		t.Errorf("function description = %v, want %q", fn["description"], "Get weather")
	}
	params, ok := fn["parameters"].(map[string]interface{})
	if !ok || params["type"] != "object" {
		t.Errorf("function parameters = %v, want the input_schema object translated verbatim", fn["parameters"])
	}
}

// TestToOpenAIChatRequest_AssistantToolUseAndToolResultTranslation
// verifies Anthropic's structured content-block messages (assistant
// tool_use + user tool_result) translate to OpenAI's tool_calls / tool
// role message shape.
func TestToOpenAIChatRequest_AssistantToolUseAndToolResultTranslation(t *testing.T) {
	in := []byte(`{
		"model": "m",
		"messages": [
			{"role":"user","content":"what's the weather in NYC?"},
			{"role":"assistant","content":[
				{"type":"text","text":"Let me check."},
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"location":"NYC"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"Sunny, 72F"}
			]}
		]
	}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]interface{})
	if len(msgs) != 3 {
		t.Fatalf("messages = %v, want 3 entries", msgs)
	}

	assistantMsg := msgs[1].(map[string]interface{})
	if assistantMsg["role"] != "assistant" {
		t.Fatalf("messages[1].role = %v, want assistant", assistantMsg["role"])
	}
	if assistantMsg["content"] != "Let me check." {
		t.Errorf("messages[1].content = %v, want the text block content", assistantMsg["content"])
	}
	toolCalls, ok := assistantMsg["tool_calls"].([]interface{})
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("messages[1].tool_calls = %v, want 1 entry", assistantMsg["tool_calls"])
	}
	tc := toolCalls[0].(map[string]interface{})
	if tc["id"] != "toolu_1" {
		t.Errorf("tool_calls[0].id = %v, want toolu_1", tc["id"])
	}
	fn := tc["function"].(map[string]interface{})
	if fn["name"] != "get_weather" {
		t.Errorf("tool_calls[0].function.name = %v, want get_weather", fn["name"])
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(fn["arguments"].(string)), &args); err != nil {
		t.Fatalf("tool_calls[0].function.arguments is not valid JSON: %v", err)
	}
	if args["location"] != "NYC" {
		t.Errorf("parsed arguments = %v, want location=NYC", args)
	}

	toolResultMsg := msgs[2].(map[string]interface{})
	if toolResultMsg["role"] != "tool" {
		t.Errorf("messages[2].role = %v, want tool", toolResultMsg["role"])
	}
	if toolResultMsg["tool_call_id"] != "toolu_1" {
		t.Errorf("messages[2].tool_call_id = %v, want toolu_1", toolResultMsg["tool_call_id"])
	}
	if toolResultMsg["content"] != "Sunny, 72F" {
		t.Errorf("messages[2].content = %v, want the tool_result content", toolResultMsg["content"])
	}
}

// TestToOpenAIChatRequest_MalformedJSONIsError verifies malformed input is
// rejected explicitly.
func TestToOpenAIChatRequest_MalformedJSONIsError(t *testing.T) {
	_, err := ToOpenAIChatRequest([]byte(`{not json`), "m")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestToOpenAIChatRequest_UsesCallerModelNotBodyModel is a regression test
// for a live-testing-discovered bug: the outbound "model" field MUST come
// from the caller-supplied model param (the catalog-resolved backend model
// id), NOT from the request body's own "model" field. The body's model is
// the client-facing catalog reference (e.g. "codex-local/gpt-5.3-codex-spark"
// for a direct <group>/<model> reference) — sending it verbatim to the
// backend causes the backend to reject an unknown model name.
func TestToOpenAIChatRequest_UsesCallerModelNotBodyModel(t *testing.T) {
	in := []byte(`{"model":"codex-local/gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}]}`)
	out, err := ToOpenAIChatRequest(in, "gpt-5.3-codex-spark")
	if err != nil {
		t.Fatalf("ToOpenAIChatRequest() error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["model"] != "gpt-5.3-codex-spark" {
		t.Errorf("model = %v, want gpt-5.3-codex-spark (the resolved model param, not the body's group-prefixed reference)", got["model"])
	}
}

// TestFromOpenAIChatResponse_TextOnly verifies a plain-text OpenAI chat
// completion response translates to an Anthropic message response.
func TestFromOpenAIChatResponse_TextOnly(t *testing.T) {
	in := []byte(`{
		"id": "chatcmpl-1",
		"choices": [{"index":0,"message":{"role":"assistant","content":"Hello there"},"finish_reason":"stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 3}
	}`)
	out, err := FromOpenAIChatResponse(in, "m")
	if err != nil {
		t.Fatalf("FromOpenAIChatResponse() error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["type"] != "message" || got["role"] != "assistant" {
		t.Errorf("type/role = %v/%v, want message/assistant", got["type"], got["role"])
	}
	content, ok := got["content"].([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("content = %v, want 1 block", got["content"])
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "text" || block["text"] != "Hello there" {
		t.Errorf("content block = %v, want text block %q", block, "Hello there")
	}
	if got["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %v, want end_turn (mapped from OpenAI's \"stop\")", got["stop_reason"])
	}
	usage := got["usage"].(map[string]interface{})
	if usage["input_tokens"] != float64(10) || usage["output_tokens"] != float64(3) {
		t.Errorf("usage = %v, want input_tokens=10 output_tokens=3", usage)
	}
}

// TestFromOpenAIChatResponse_ToolCalls verifies an OpenAI response with
// tool_calls translates each into an Anthropic tool_use block with parsed
// (structured) input, per AC-PROXY-012b's non-streaming analogue.
func TestFromOpenAIChatResponse_ToolCalls(t *testing.T) {
	in := []byte(`{
		"id": "chatcmpl-2",
		"choices": [{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"NYC\"}"}}
		]},"finish_reason":"tool_calls"}],
		"usage": {"prompt_tokens": 20, "completion_tokens": 8}
	}`)
	out, err := FromOpenAIChatResponse(in, "m")
	if err != nil {
		t.Fatalf("FromOpenAIChatResponse() error = %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	content := got["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("content = %v, want 1 tool_use block", content)
	}
	block := content[0].(map[string]interface{})
	if block["type"] != "tool_use" || block["id"] != "call_1" || block["name"] != "get_weather" {
		t.Errorf("tool_use block = %v", block)
	}
	input, ok := block["input"].(map[string]interface{})
	if !ok || input["location"] != "NYC" {
		t.Errorf("tool_use input = %v, want a parsed structured object {location: NYC}", block["input"])
	}
	if got["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v, want tool_use", got["stop_reason"])
	}
}

// TestFromOpenAIChatResponse_MalformedToolCallArgumentsIsError verifies
// unparseable tool-call arguments surface as an explicit error rather than
// a silently empty/corrupted input object.
func TestFromOpenAIChatResponse_MalformedToolCallArgumentsIsError(t *testing.T) {
	in := []byte(`{
		"choices": [{"index":0,"message":{"role":"assistant","tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"f","arguments":"{not json"}}
		]},"finish_reason":"tool_calls"}]
	}`)
	_, err := FromOpenAIChatResponse(in, "m")
	if err == nil {
		t.Fatal("expected error for malformed tool-call arguments JSON, got nil")
	}
}

// TestFromOpenAIChatResponse_FinishReasonMapping verifies the
// finish_reason -> stop_reason mapping table.
func TestFromOpenAIChatResponse_FinishReasonMapping(t *testing.T) {
	cases := []struct {
		openai string
		want   string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
	}
	for _, c := range cases {
		in := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"x"},"finish_reason":"` + c.openai + `"}]}`)
		out, err := FromOpenAIChatResponse(in, "m")
		if err != nil {
			t.Fatalf("finish_reason=%q: FromOpenAIChatResponse() error = %v", c.openai, err)
		}
		var got map[string]interface{}
		_ = json.Unmarshal(out, &got)
		if got["stop_reason"] != c.want {
			t.Errorf("finish_reason=%q: stop_reason = %v, want %v", c.openai, got["stop_reason"], c.want)
		}
	}
}

// TestFromOpenAIChatResponse_MalformedJSONIsError verifies malformed input
// is rejected explicitly.
func TestFromOpenAIChatResponse_MalformedJSONIsError(t *testing.T) {
	_, err := FromOpenAIChatResponse([]byte(`{not json`), "m")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
