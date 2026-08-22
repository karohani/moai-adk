package proxy

// Regression tests for the defects found in the 2026-08-22 architecture
// review (Claude + codex cross-audit). Each test encodes the CORRECTED
// behavior and fails against the pre-fix implementation.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// B1: the daemon state file and its directory must not be group/world
// readable — the daemon's address is a capability handle for every backend
// credential it holds, and the state file is also a hijack surface (an
// attacker-planted RefCount+Address redirects Claude Code's
// ANTHROPIC_BASE_URL).
func TestDaemonStateFileIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "proxy")
	d := NewDaemon(stateDir)

	addr, err := d.Acquire(func() (string, func() error, error) {
		return "127.0.0.1:65001", func() error { return nil }, nil
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if addr == "" {
		t.Fatal("empty address")
	}

	stInfo, err := os.Stat(filepath.Join(stateDir, daemonStateFileName))
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if perm := stInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("state file mode = %04o, want no group/world bits (0600)", perm)
	}

	dirInfo, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("stat state dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("state dir mode = %04o, want no group/world bits (0700)", perm)
	}
}

// B1/N2/H2: a state file naming a dead daemon must NOT be reused. Without a
// liveness check a crashed holder poisons the machine-scope state forever
// (RefCount never returns to 0), and every later session is handed an
// address nothing is listening on.
func TestAcquireRejectsStaleStateAndStartsFresh(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "proxy")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A state file left behind by a crashed daemon: positive refcount,
	// an address nothing listens on, and a PID that is not running.
	stale := DaemonState{PID: 999999, Address: "127.0.0.1:9", RefCount: 3}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(filepath.Join(stateDir, daemonStateFileName), data, 0o600); err != nil {
		t.Fatal(err)
	}

	d := NewDaemon(stateDir)
	started := false
	addr, err := d.Acquire(func() (string, func() error, error) {
		started = true
		return "127.0.0.1:65002", func() error { return nil }, nil
	})
	if err != nil {
		t.Fatalf("Acquire over stale state: %v", err)
	}
	if !started {
		t.Fatal("factory was not invoked — a dead daemon's address was reused")
	}
	if addr != "127.0.0.1:65002" {
		t.Errorf("address = %q, want the freshly started server", addr)
	}
}

// H4: tool_choice carries a semantically load-bearing constraint (force a
// tool, forbid tools). Dropping it silently makes the backend fall back to
// "auto" and answer in prose while the client believes its constraint held.
func TestToOpenAIChatRequest_TranslatesToolChoice(t *testing.T) {
	cases := []struct {
		name      string
		anthropic string
		want      interface{}
	}{
		{"auto", `{"type":"auto"}`, "auto"},
		{"any", `{"type":"any"}`, "required"},
		{"none", `{"type":"none"}`, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(`{"messages":[],"tools":[{"name":"f","input_schema":{}}],"tool_choice":` + tc.anthropic + `}`)
			out, err := ToOpenAIChatRequest(in, "m")
			if err != nil {
				t.Fatalf("err %v", err)
			}
			var got map[string]interface{}
			_ = json.Unmarshal(out, &got)
			if got["tool_choice"] != tc.want {
				t.Errorf("tool_choice = %#v, want %#v", got["tool_choice"], tc.want)
			}
		})
	}

	t.Run("named tool", func(t *testing.T) {
		in := []byte(`{"messages":[],"tools":[{"name":"get_weather","input_schema":{}}],"tool_choice":{"type":"tool","name":"get_weather"}}`)
		out, err := ToOpenAIChatRequest(in, "m")
		if err != nil {
			t.Fatalf("err %v", err)
		}
		var got map[string]interface{}
		_ = json.Unmarshal(out, &got)
		tc, ok := got["tool_choice"].(map[string]interface{})
		if !ok {
			t.Fatalf("tool_choice = %#v, want an object", got["tool_choice"])
		}
		fn, ok := tc["function"].(map[string]interface{})
		if !ok || tc["type"] != "function" || fn["name"] != "get_weather" {
			t.Errorf("tool_choice = %#v, want {type:function,function:{name:get_weather}}", tc)
		}
	})
}

// H5: content blocks carrying USER intent must never be silently dropped.
// Images are TRANSLATED (OpenAI has an image_url content part); a block type
// with no representation at all is an explicit error naming the type.
func TestToOpenAIChatRequest_ImageBlockIsTranslated(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[
		{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBOR"}},
		{"type":"text","text":"what is this?"}
	]}]}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("image block should translate, got error: %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]interface{})
	parts, ok := msgs[0].(map[string]interface{})["content"].([]interface{})
	if !ok {
		t.Fatalf("content = %#v, want a multimodal parts array", msgs[0])
	}
	var sawText, sawImage bool
	for _, p := range parts {
		pm := p.(map[string]interface{})
		switch pm["type"] {
		case "text":
			sawText = pm["text"] == "what is this?"
		case "image_url":
			iu := pm["image_url"].(map[string]interface{})
			sawImage = iu["url"] == "data:image/png;base64,iVBOR"
		}
	}
	if !sawText || !sawImage {
		t.Errorf("parts = %#v, want both the text and a data-URI image_url", parts)
	}
}

// A block type with genuinely no OpenAI representation still errors, naming
// the type so the caller can route the request elsewhere.
func TestToOpenAIChatRequest_UntranslatableUserBlockIsError(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[
		{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"JVBER"}}
	]}]}`)
	_, err := ToOpenAIChatRequest(in, "m")
	if err == nil {
		t.Fatal("expected an error for an untranslatable user block, got nil (silent drop)")
	}
	if !strings.Contains(err.Error(), "document") {
		t.Errorf("error %q should name the offending block type", err)
	}
}

// REGRESSION GUARD: Anthropic REQUIRES thinking blocks to be echoed back in
// message history once extended thinking is on. Erroring on them would break
// every multi-turn session against an OpenAI-shaped backend from turn two.
func TestToOpenAIChatRequest_AssistantInternalBlocksAreSkipped(t *testing.T) {
	in := []byte(`{"messages":[
		{"role":"user","content":"2+2?"},
		{"role":"assistant","content":[
			{"type":"thinking","thinking":"compute","signature":"sig"},
			{"type":"text","text":"4"}
		]},
		{"role":"user","content":"3+3?"}
	]}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("thinking block in history must not fail the request: %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]interface{})
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3 turns preserved", len(msgs))
	}
	if c := msgs[1].(map[string]interface{})["content"]; c != "4" {
		t.Errorf("assistant content = %v, want the text block preserved alongside the skipped thinking", c)
	}
}

// M4/M6: the non-streaming response must carry an array `content` (never
// null), the resolved `model`, and `stop_sequence` — the streaming path
// already emits model, so omitting it here leaves the two paths inconsistent.
func TestFromOpenAIChatResponse_ShapeIsAnthropicComplete(t *testing.T) {
	in := []byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":0}}`)
	out, err := FromOpenAIChatResponse(in, "resolved-model")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got["content"] == nil {
		t.Error("content = null, want an empty array []")
	}
	if got["model"] != "resolved-model" {
		t.Errorf("model = %v, want the resolved model", got["model"])
	}
	if _, ok := got["stop_sequence"]; !ok {
		t.Error("stop_sequence field missing")
	}
}

// M5: an OpenAI finish_reason with no Anthropic equivalent must not be
// passed through verbatim — "content_filter" is not a member of Anthropic's
// stop_reason enum and strict clients reject it.
func TestFromOpenAIChatResponse_UnmappedFinishReasonIsCoerced(t *testing.T) {
	in := []byte(`{"id":"c","choices":[{"message":{"role":"assistant","content":"x"},"finish_reason":"content_filter"}],"usage":{}}`)
	out, err := FromOpenAIChatResponse(in, "m")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	valid := map[string]bool{"end_turn": true, "max_tokens": true, "stop_sequence": true, "tool_use": true, "refusal": true, "pause_turn": true}
	if !valid[got["stop_reason"].(string)] {
		t.Errorf("stop_reason = %q, which is not a valid Anthropic stop_reason", got["stop_reason"])
	}
}

// B5: the litellm path must send the RESOLVED model. REQ-PROXY-017's
// "no translation" governs the request shape, not the model identifier —
// relaying `opus` or `llm-a/gpt-4o` verbatim forwards a name this proxy
// invented and the backend has never heard of.
func TestServeLiteLLM_SendsResolvedModel(t *testing.T) {
	cases := []struct {
		name       string
		clientSent string
		wantModel  string
	}{
		{"direct group/model reference", "llm-a/gpt-4o", "gpt-4o"},
		{"alias", "opus", "real-opus-model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seenModel string
			var seenExtra string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var got map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&got)
				seenModel, _ = got["model"].(string)
				seenExtra, _ = got["custom_passthrough"].(string)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"x","type":"message","role":"assistant","content":[]}`))
			}))
			defer backend.Close()

			reg := &Registry{
				Groups: map[string]Group{
					"llm-a": {Type: GroupTypeLiteLLM, BaseURL: backend.URL, Aliases: AliasBlock{Opus: "real-opus-model"}},
				},
				Sets: map[string][]string{},
			}
			h, err := NewMessagesHandler(reg, NewCatalog(reg, []string{"llm-a"}), nil)
			if err != nil {
				t.Fatalf("handler: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(
				`{"model":"`+tc.clientSent+`","custom_passthrough":"keep-me","messages":[{"role":"user","content":"hi"}]}`))
			h.ServeHTTP(httptest.NewRecorder(), req)

			if seenModel != tc.wantModel {
				t.Errorf("backend received model %q, want %q", seenModel, tc.wantModel)
			}
			if seenExtra != "keep-me" {
				t.Errorf("passthrough field lost: custom_passthrough = %q, want keep-me", seenExtra)
			}
		})
	}
}

// B3: id and name arriving in SEPARATE chunks must still produce a complete
// tool_use block, with argument fragments that preceded the identity
// flushed in order rather than dropped.
func TestStream_ToolCallIdentitySplitAcrossChunks(t *testing.T) {
	raw := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"arguments":""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"get_weather","arguments":"\"NYC\"}"}}]}}]}
data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(raw), "msg_a", "m")
	out, _ := io.ReadAll(rc)
	got := string(out)

	if !strings.Contains(got, `"type":"tool_use"`) {
		t.Fatalf("no tool_use content block emitted:\n%s", got)
	}
	if !strings.Contains(got, `"name":"get_weather"`) || !strings.Contains(got, `"id":"call_1"`) {
		t.Errorf("tool_use block missing accumulated id/name:\n%s", got)
	}
	// Both argument fragments must survive, head first.
	headAt := strings.Index(got, `{\"city\":`)
	tailAt := strings.Index(got, `\"NYC\"}`)
	if headAt < 0 || tailAt < 0 {
		t.Errorf("argument fragments lost (head=%d tail=%d):\n%s", headAt, tailAt, got)
	} else if headAt > tailAt {
		t.Errorf("argument fragments emitted out of order:\n%s", got)
	}
}

// B4: a stream truncated before any finish_reason must NOT be reported as a
// normal end_turn completion.
func TestStream_TruncationIsSignalled(t *testing.T) {
	raw := "data: {\"choices\":[{\"delta\":{\"content\":\"Hello, the answer is\"}}]}\n"
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(raw), "msg_x", "m")
	out, _ := io.ReadAll(rc)
	got := string(out)

	if !strings.Contains(got, "event: error") {
		t.Errorf("truncated stream carries no error event:\n%s", got)
	}
	if strings.Contains(got, `"stop_reason":"end_turn"`) {
		t.Errorf("truncated stream reported stop_reason end_turn (indistinguishable from success):\n%s", got)
	}
	// The partial content the backend did send is still relayed.
	if !strings.Contains(got, "Hello, the answer is") {
		t.Errorf("partial content dropped:\n%s", got)
	}
}

// B4 (converse): a NORMAL stream must not gain an error event.
func TestStream_NormalCompletionHasNoErrorEvent(t *testing.T) {
	raw := `data: {"choices":[{"delta":{"content":"hi"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(raw), "msg_n", "m")
	out, _ := io.ReadAll(rc)
	got := string(out)
	if strings.Contains(got, "event: error") {
		t.Errorf("normal stream gained a spurious error event:\n%s", got)
	}
	if !strings.Contains(got, `"stop_reason":"end_turn"`) {
		t.Errorf("normal stream lost its end_turn stop_reason:\n%s", got)
	}
}

// H8: usage must be requested AND reported. input_tokens hardcoded to 0
// silently corrupted every streaming cost/context calculation.
func TestStream_ReportsInputTokens(t *testing.T) {
	raw := `data: {"choices":[{"delta":{"content":"hi"}}]}
data: {"choices":[{"delta":{},"finish_reason":"stop"}]}
data: {"choices":[],"usage":{"prompt_tokens":1234,"completion_tokens":56}}
data: [DONE]
`
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(raw), "msg_u", "m")
	out, _ := io.ReadAll(rc)
	got := string(out)
	if !strings.Contains(got, `"input_tokens":1234`) {
		t.Errorf("message_delta lost the backend's prompt_tokens:\n%s", got)
	}
	if !strings.Contains(got, `"output_tokens":56`) {
		t.Errorf("message_delta lost completion_tokens:\n%s", got)
	}
}

// H8: the outbound request must ASK for the usage chunk, or a
// spec-conforming backend never sends one.
func TestToOpenAIChatRequest_StreamingRequestsUsage(t *testing.T) {
	out, err := ToOpenAIChatRequest([]byte(`{"messages":[],"stream":true}`), "m")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	so, ok := got["stream_options"].(map[string]interface{})
	if !ok || so["include_usage"] != true {
		t.Errorf("stream_options = %#v, want {include_usage:true}", got["stream_options"])
	}

	// Non-streaming requests must NOT carry it.
	out2, _ := ToOpenAIChatRequest([]byte(`{"messages":[]}`), "m")
	var got2 map[string]interface{}
	_ = json.Unmarshal(out2, &got2)
	if _, present := got2["stream_options"]; present {
		t.Error("stream_options must be absent on a non-streaming request")
	}
}

// H1: with TWO bedrock groups registered, each request must reach the
// invoker belonging to the group the catalog resolved. The pre-fix design
// held one shared invoker picked by Go's randomized map iteration and used
// it for every bedrock request, so a request for group B was signed with
// group A's region and profile — and which group won changed between
// daemon restarts.
func TestServeBedrock_UsesTheResolvedGroupsInvoker(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{
			"br-us": {Type: GroupTypeBedrock, Region: "us-east-1", Profile: "prod"},
			"br-eu": {Type: GroupTypeBedrock, Region: "eu-central-1", Profile: "staging"},
		},
		Sets: map[string][]string{},
	}
	us := &fakeBedrockInvoker{invokeResp: []byte(`{"id":"from-us","content":[]}`)}
	eu := &fakeBedrockInvoker{invokeResp: []byte(`{"id":"from-eu","content":[]}`)}

	h, err := NewMessagesHandler(reg, NewCatalog(reg, []string{"br-us", "br-eu"}),
		map[string]BedrockInvoker{"br-us": us, "br-eu": eu})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	// Route explicitly to the EU group.
	req := httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"br-eu/anthropic.claude-x","messages":[],"max_tokens":10}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body.String())
	}
	if eu.invokeCalls != 1 {
		t.Errorf("eu invoker called %d times, want 1", eu.invokeCalls)
	}
	if us.invokeCalls != 0 {
		t.Errorf("us invoker called %d times, want 0 — request was signed by the WRONG group", us.invokeCalls)
	}
	if !strings.Contains(w.Body.String(), "from-eu") {
		t.Errorf("response came from the wrong invoker: %s", w.Body.String())
	}
}

// H1: a bedrock group with no invoker configured must name ITSELF in the
// error, not fail generically — with per-group invokers the operator needs
// to know which group is unconfigured.
func TestServeBedrock_MissingInvokerNamesTheGroup(t *testing.T) {
	reg := &Registry{
		Groups: map[string]Group{"br-us": {Type: GroupTypeBedrock, Region: "us-east-1"}},
		Sets:   map[string][]string{},
	}
	h, err := NewMessagesHandler(reg, NewCatalog(reg, []string{"br-us"}), nil)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"br-us/m","messages":[]}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "br-us") {
		t.Errorf("error does not name the group: %s", w.Body.String())
	}
}

// H7: backend errors must preserve actionable status semantics, must not
// echo the upstream body verbatim, and must be Anthropic-shaped JSON.
func TestBackendError_PreservesStatusAndRedactsEnvelope(t *testing.T) {
	cases := []struct {
		upstream    int
		wantStatus  int
		wantErrType string
	}{
		{http.StatusUnauthorized, http.StatusUnauthorized, "authentication_error"},
		{http.StatusTooManyRequests, http.StatusTooManyRequests, "rate_limit_error"},
		{http.StatusBadRequest, http.StatusBadRequest, "invalid_request_error"},
		{http.StatusNotFound, http.StatusNotFound, "not_found_error"},
		{http.StatusInternalServerError, http.StatusBadGateway, "api_error"},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.upstream), func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.upstream)
				// A realistic provider envelope carrying a key fragment in a
				// field OUTSIDE `message`.
				_, _ = w.Write([]byte(`{"error":{"message":"the model is unavailable","type":"x","param":null,"code":"sk-live-SECRET1234"}}`))
			}))
			defer backend.Close()

			reg := &Registry{
				Groups: map[string]Group{"gpu": {Type: GroupTypeOpenAICompatible, BaseURL: backend.URL}},
				Sets:   map[string][]string{},
			}
			h, err := NewMessagesHandler(reg, NewCatalog(reg, []string{"gpu"}), nil)
			if err != nil {
				t.Fatalf("handler: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/messages",
				strings.NewReader(`{"model":"gpu/m","messages":[{"role":"user","content":"hi"}]}`))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (upstream %d)", w.Code, tc.wantStatus, tc.upstream)
			}
			if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json (Anthropic SDKs parse JSON errors)", ct)
			}
			var got map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("error body is not JSON: %v (%s)", err, w.Body.String())
			}
			if got["type"] != "error" {
				t.Errorf("body.type = %v, want \"error\"", got["type"])
			}
			e, _ := got["error"].(map[string]interface{})
			if e["type"] != tc.wantErrType {
				t.Errorf("error.type = %v, want %q", e["type"], tc.wantErrType)
			}
			// The provider's own message survives...
			if msg, _ := e["message"].(string); !strings.Contains(msg, "the model is unavailable") {
				t.Errorf("error.message = %q, want the provider message preserved", msg)
			}
			// ...but the surrounding envelope, where identifiers live, does not.
			if strings.Contains(w.Body.String(), "sk-live-SECRET1234") {
				t.Errorf("upstream envelope leaked to the client: %s", w.Body.String())
			}
		})
	}
}

// B2: base_url must be validated BEFORE a credential is attached. The
// pre-fix code concatenated the path onto whatever string the registry
// held, so a typo'd or planted value received the Codex OAuth token.
func TestBackendChatCompletionsURL(t *testing.T) {
	valid := []struct{ in, want string }{
		{"http://10.0.0.5:8000/v1", "http://10.0.0.5:8000/v1/chat/completions"},
		{"http://10.0.0.5:8000/v1/", "http://10.0.0.5:8000/v1/chat/completions"},
		{"https://api.example.com", "https://api.example.com/chat/completions"},
	}
	for _, tc := range valid {
		got, err := backendChatCompletionsURL(tc.in)
		if err != nil {
			t.Errorf("backendChatCompletionsURL(%q) error = %v, want success", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("backendChatCompletionsURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	rejected := []struct{ in, because string }{
		{"", "empty base_url"},
		{"file:///etc/passwd", "non-http scheme"},
		{"gopher://x/", "non-http scheme"},
		{"https://user:pw@host/v1", "credentials embedded in URL"},
		{"https://h/v1?key=x", "query string corrupts path joining"},
		{"/v1", "no host"},
	}
	for _, tc := range rejected {
		if _, err := backendChatCompletionsURL(tc.in); err == nil {
			t.Errorf("backendChatCompletionsURL(%q) succeeded, want rejection (%s)", tc.in, tc.because)
		}
	}
}

// B2: the credential must never leave the process when base_url is invalid —
// the request must fail before any backend call is attempted.
func TestServeOpenAIShaped_InvalidBaseURLFailsBeforeAuth(t *testing.T) {
	authAttached := false
	auth := func(*http.Request) error { authAttached = true; return nil }

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	serveOpenAIShapedMessages(w, req, &http.Client{}, "file:///etc/passwd", auth,
		[]byte(`{"messages":[],"model":"m"}`), "m", false)

	if authAttached {
		t.Error("auth closure ran for an invalid base_url — the credential was prepared for an unvalidated destination")
	}
	if w.Code < 400 {
		t.Errorf("status = %d, want an error status", w.Code)
	}
}

// Unit coverage for the error-translation helpers across the branches the
// end-to-end test does not reach.
func TestBackendErrorHelpers(t *testing.T) {
	t.Run("status mapping", func(t *testing.T) {
		cases := map[int]int{
			http.StatusForbidden:             http.StatusForbidden,
			http.StatusRequestTimeout:        http.StatusRequestTimeout,
			http.StatusConflict:              http.StatusConflict,
			http.StatusRequestEntityTooLarge: http.StatusRequestEntityTooLarge,
			http.StatusUnprocessableEntity:   http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable:    http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:        http.StatusGatewayTimeout,
			http.StatusNotImplemented:        http.StatusBadGateway,
			http.StatusInsufficientStorage:   http.StatusBadGateway,
		}
		for upstream, want := range cases {
			if got := mapBackendStatus(upstream); got != want {
				t.Errorf("mapBackendStatus(%d) = %d, want %d", upstream, got, want)
			}
		}
	})

	t.Run("error type vocabulary", func(t *testing.T) {
		cases := map[int]string{
			http.StatusForbidden:             "permission_error",
			http.StatusRequestEntityTooLarge: "request_too_large",
			http.StatusServiceUnavailable:    "overloaded_error",
			http.StatusGatewayTimeout:        "overloaded_error",
			http.StatusBadGateway:            "api_error",
		}
		for status, want := range cases {
			if got := anthropicErrorType(status); got != want {
				t.Errorf("anthropicErrorType(%d) = %q, want %q", status, got, want)
			}
		}
	})

	t.Run("message extraction", func(t *testing.T) {
		cases := []struct{ raw, want string }{
			{``, "backend returned an error with no body"},
			{`{"error":{"message":"nested form"}}`, "nested form"},
			{`{"message":"flat form"}`, "flat form"},
			{`{"detail":"detail form"}`, "detail form"},
			{`not json at all`, "backend returned an unparseable error response"},
			{`{"error":{}}`, "backend returned an unparseable error response"},
		}
		for _, tc := range cases {
			if got := backendErrorMessage([]byte(tc.raw)); got != tc.want {
				t.Errorf("backendErrorMessage(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		}
	})

	t.Run("oversized body is truncated, not buffered whole", func(t *testing.T) {
		huge := strings.Repeat("A", backendErrorBodyLimit*4)
		w := httptest.NewRecorder()
		relayBackendError(w, http.StatusInternalServerError, strings.NewReader(huge))
		if w.Body.Len() > backendErrorBodyLimit {
			t.Errorf("response body = %d bytes, want the upstream body bounded", w.Body.Len())
		}
	})
}

// daemonStateIsLive must reject an empty address and a dead one, and accept
// a live listener.
func TestDaemonStateIsLive(t *testing.T) {
	if daemonStateIsLive(DaemonState{Address: ""}) {
		t.Error("empty address reported live")
	}
	// Port 9 (discard) is conventionally closed on developer machines; a
	// dead address must not be reported live.
	if daemonStateIsLive(DaemonState{Address: "127.0.0.1:9"}) {
		t.Error("unreachable address reported live")
	}

	ln := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ln.Close()
	addr := strings.TrimPrefix(ln.URL, "http://")
	if !daemonStateIsLive(DaemonState{Address: addr}) {
		t.Errorf("live listener at %s reported dead", addr)
	}
}

// translateToolChoice must reject shapes it cannot map rather than guessing.
func TestTranslateToolChoice_UnmappableShapesAreOmitted(t *testing.T) {
	for _, raw := range []string{``, `not json`, `{"type":"wat"}`, `{"type":"tool"}`} {
		if v, ok := translateToolChoice([]byte(raw)); ok {
			t.Errorf("translateToolChoice(%q) = %v, true; want omission", raw, v)
		}
	}
}

// A stream that dies BEFORE its first chunk must still tell the client. The
// HTTP layer writes 200 and the SSE headers before reading the first byte,
// so silence there leaves a successful-looking response with an empty body.
func TestCloseTruncated_UnstartedStreamStillSignals(t *testing.T) {
	tr := newOpenAIStreamTranslator("msg", "m")
	frames := tr.CloseTruncated("connection reset before first chunk")
	joined := ""
	for _, f := range frames {
		joined += string(f)
	}
	if !strings.Contains(joined, "event: message_start") {
		t.Errorf("no message_start — the error frame must sit inside a well-formed stream:\n%s", joined)
	}
	if !strings.Contains(joined, "event: error") {
		t.Errorf("no error event for a stream that died before any chunk:\n%s", joined)
	}
	if strings.Contains(joined, `"stop_reason":"end_turn"`) {
		t.Errorf("reported end_turn for a stream that produced nothing:\n%s", joined)
	}
}

// A [DONE] sentinel with no preceding finish_reason is a truncation, not a
// natural completion — finalize() would have reported end_turn.
func TestStream_DoneWithoutFinishReasonIsTruncation(t *testing.T) {
	raw := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\ndata: [DONE]\n"
	rc := TranslateOpenAIStreamToAnthropicSSE(strings.NewReader(raw), "m1", "m")
	out, _ := io.ReadAll(rc)
	got := string(out)
	if !strings.Contains(got, "event: error") {
		t.Errorf("[DONE] without finish_reason produced no error event:\n%s", got)
	}
	if strings.Contains(got, `"stop_reason":"end_turn"`) {
		t.Errorf("[DONE] without finish_reason reported end_turn:\n%s", got)
	}
	if !strings.Contains(got, "partial") {
		t.Errorf("partial content dropped:\n%s", got)
	}
}

// A stream whose backend DID report a finish reason before the read error
// terminates normally — the error is incidental.
func TestCloseTruncated_AfterFinishReasonTerminatesNormally(t *testing.T) {
	tr := newOpenAIStreamTranslator("msg", "m")
	tr.Feed([]byte(`data: {"choices":[{"delta":{"content":"hi"}}]}`))
	tr.Feed([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`))
	frames := tr.CloseTruncated("connection reset")
	joined := ""
	for _, f := range frames {
		joined += string(f)
	}
	if strings.Contains(joined, "event: error") {
		t.Errorf("a stream with a finish_reason gained a spurious error event:\n%s", joined)
	}
	if !strings.Contains(joined, `"stop_reason":"end_turn"`) {
		t.Errorf("expected the reported end_turn to survive:\n%s", joined)
	}
}

// Cross-audit round 2 regressions.

// F4: an explicit -g/--set must not be blocked by a malformed project
// config it never consults.
func TestResolveActiveGroups_ExplicitFlagIgnoresProjectConfig(t *testing.T) {
	t.Skip("covered in internal/cli; see TestResolveProxyActiveGroups_ExplicitFlagSkipsProjectConfig")
}

// F1b: an install predating the tightened modes must be repaired, not just
// created correctly next time.
func TestDaemon_TightensPreExistingPermissiveModes(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "proxy")
	if err := os.MkdirAll(stateDir, 0o755); err != nil { // legacy mode
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, daemonStateFileName), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDaemon(stateDir)
	if _, err := d.Acquire(func() (string, func() error, error) {
		return "127.0.0.1:65010", func() error { return nil }, nil
	}); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	di, _ := os.Stat(stateDir)
	if p := di.Mode().Perm(); p&0o077 != 0 {
		t.Errorf("pre-existing state dir left at %04o — legacy installs stay exposed", p)
	}
	fi, _ := os.Stat(filepath.Join(stateDir, daemonStateFileName))
	if p := fi.Mode().Perm(); p&0o077 != 0 {
		t.Errorf("pre-existing state file left at %04o — legacy installs stay exposed", p)
	}
}

// F11: the provider's message field itself carries key fragments.
func TestBackendErrorMessage_RedactsSecretShapedTokens(t *testing.T) {
	cases := []struct{ raw, mustNotContain string }{
		{`{"error":{"message":"Incorrect API key provided: sk-live-ABCDEFGH1234"}}`, "sk-live-ABCDEFGH1234"},
		{`{"error":{"message":"bad token: Bearer eyJhbGciOiJIUzI1NiJ9abcdefg"}}`, "eyJhbGciOiJIUzI1NiJ9abcdefg"},
		{`{"message":"rejected key ghp_ABCDEFGHIJKLMNOP"}`, "ghp_ABCDEFGHIJKLMNOP"},
	}
	for _, tc := range cases {
		got := backendErrorMessage([]byte(tc.raw))
		if strings.Contains(got, tc.mustNotContain) {
			t.Errorf("message %q still carries the secret-shaped token %q", got, tc.mustNotContain)
		}
		if !strings.Contains(got, "[redacted]") {
			t.Errorf("message %q should mark the redaction", got)
		}
	}
	// A benign message survives intact.
	if got := backendErrorMessage([]byte(`{"error":{"message":"model is overloaded"}}`)); got != "model is overloaded" {
		t.Errorf("benign message altered: %q", got)
	}
}

// F2b: the link-local/metadata range is never a valid backend.
func TestBackendChatCompletionsURL_RejectsMetadataRange(t *testing.T) {
	for _, u := range []string{"http://169.254.169.254", "http://169.254.169.254/v1", "http://[fe80::1]/v1"} {
		if _, err := backendChatCompletionsURL(u); err == nil {
			t.Errorf("backendChatCompletionsURL(%q) succeeded, want rejection", u)
		}
	}
	// An ordinary private-network cluster (the documented example) still works.
	if _, err := backendChatCompletionsURL("http://10.0.0.5:8000/v1"); err != nil {
		t.Errorf("a private-range cluster must remain valid: %v", err)
	}
}

// disable_parallel_tool_use maps to OpenAI's sibling field.
func TestToOpenAIChatRequest_DisableParallelToolUse(t *testing.T) {
	in := []byte(`{"messages":[],"tools":[{"name":"f","input_schema":{}}],"tool_choice":{"type":"auto","disable_parallel_tool_use":true}}`)
	out, err := ToOpenAIChatRequest(in, "m")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(out, &got)
	if got["parallel_tool_calls"] != false {
		t.Errorf("parallel_tool_calls = %#v, want false", got["parallel_tool_calls"])
	}
	// Absent flag must not emit the field at all.
	out2, _ := ToOpenAIChatRequest([]byte(`{"messages":[],"tool_choice":{"type":"auto"}}`), "m")
	var got2 map[string]interface{}
	_ = json.Unmarshal(out2, &got2)
	if _, present := got2["parallel_tool_calls"]; present {
		t.Error("parallel_tool_calls emitted when the caller did not set the flag")
	}
}

// F12: values are preserved exactly (no number normalization); key order is
// not, and the comment must not claim otherwise.
func TestReplaceModelField_PreservesValueFidelity(t *testing.T) {
	in := []byte(`{"model":"g/m","big":12345678901234567890,"f":1.10,"nested":{"a":[1,2,3]}}`)
	out, err := replaceModelField(in, "m")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, must := range []string{`"model":"m"`, `12345678901234567890`, `1.10`, `"nested":{"a":[1,2,3]}`} {
		if !strings.Contains(s, must) {
			t.Errorf("output lost %s: %s", must, s)
		}
	}
}
