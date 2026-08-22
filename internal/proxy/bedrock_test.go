package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestToBedrockInvokeModelBody_AddsVersionRemovesModelAndStream verifies
// the decided v1 Bedrock translation (design.md §5.4 decision, recorded in
// progress.md §E.2 M2 field-mapping table): InvokeModel for Anthropic-family
// models takes the Anthropic request body nearly verbatim — add
// anthropic_version, strip model (goes in the ModelId API parameter) and
// stream (chosen by which SDK operation is invoked, not a body field).
// Every other field (messages, system, max_tokens, ...) passes through
// unchanged.
func TestToBedrockInvokeModelBody_AddsVersionRemovesModelAndStream(t *testing.T) {
	in := []byte(`{
		"model": "anthropic.claude-3-5-sonnet-20241022-v2:0",
		"stream": true,
		"messages": [{"role":"user","content":"hi"}],
		"system": "be terse",
		"max_tokens": 1024,
		"temperature": 0.7,
		"stop_sequences": ["END"]
	}`)

	out, err := ToBedrockInvokeModelBody(in)
	if err != nil {
		t.Fatalf("ToBedrockInvokeModelBody() error = %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, present := got["model"]; present {
		t.Error("model field must be removed (goes in ModelId API parameter, not body)")
	}
	if _, present := got["stream"]; present {
		t.Error("stream field must be removed (chosen by SDK operation, not body field)")
	}
	if got["anthropic_version"] != BedrockAnthropicVersion {
		t.Errorf("anthropic_version = %v, want %q", got["anthropic_version"], BedrockAnthropicVersion)
	}
	if got["system"] != "be terse" {
		t.Errorf("system field must pass through unchanged, got %v", got["system"])
	}
	if got["max_tokens"] != float64(1024) {
		t.Errorf("max_tokens field must pass through unchanged, got %v", got["max_tokens"])
	}
	msgs, ok := got["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Errorf("messages field must pass through unchanged, got %v", got["messages"])
	}
}

// TestToBedrockInvokeModelBody_MalformedJSONIsError verifies malformed
// input surfaces an error rather than silently producing a broken body.
func TestToBedrockInvokeModelBody_MalformedJSONIsError(t *testing.T) {
	_, err := ToBedrockInvokeModelBody([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestFromBedrockInvokeModelBody_IsIdentity verifies the return-path
// translation is the identity function: Bedrock's InvokeModel response body
// for Anthropic-family models IS the Anthropic response shape verbatim.
func TestFromBedrockInvokeModelBody_IsIdentity(t *testing.T) {
	in := []byte(`{"id":"msg_1","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`)
	out := FromBedrockInvokeModelBody(in)
	if !bytes.Equal(in, out) {
		t.Errorf("FromBedrockInvokeModelBody() = %s, want identity %s", out, in)
	}
}

// fakeBedrockInvoker is a hermetic test double for BedrockInvoker — no AWS
// credentials, no network, no SigV4 signing exercised (per the deliberate
// scope boundary: the real SDK adapter's signing path is never unit
// tested, only this translation/routing logic is).
type fakeBedrockInvoker struct {
	invokeCalls       int
	invokeStreamCalls int
	gotModelID        string
	gotBody           []byte
	invokeResp        []byte
	invokeErr         error
	streamResp        string
	streamErr         error
}

func (f *fakeBedrockInvoker) Invoke(_ context.Context, modelID string, body []byte) ([]byte, error) {
	f.invokeCalls++
	f.gotModelID = modelID
	f.gotBody = body
	return f.invokeResp, f.invokeErr
}

func (f *fakeBedrockInvoker) InvokeStream(_ context.Context, modelID string, body []byte) (io.ReadCloser, error) {
	f.invokeStreamCalls++
	f.gotModelID = modelID
	f.gotBody = body
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return io.NopCloser(strings.NewReader(f.streamResp)), nil
}

// TestInvokeModelViaBedrock_TranslatesAndDelegates verifies the end-to-end
// non-streaming path: translate request -> invoke -> identity-translate
// response.
func TestInvokeModelViaBedrock_TranslatesAndDelegates(t *testing.T) {
	fake := &fakeBedrockInvoker{
		invokeResp: []byte(`{"id":"msg_1","content":[{"type":"text","text":"hi"}]}`),
	}
	reqBody := []byte(`{"model":"anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`)

	got, err := InvokeModelViaBedrock(context.Background(), fake, "anthropic.claude-3-5-sonnet-20241022-v2:0", reqBody)
	if err != nil {
		t.Fatalf("InvokeModelViaBedrock() error = %v", err)
	}
	if fake.invokeCalls != 1 {
		t.Errorf("Invoke called %d times, want 1", fake.invokeCalls)
	}
	if fake.gotModelID != "anthropic.claude-3-5-sonnet-20241022-v2:0" {
		t.Errorf("gotModelID = %q, want the Bedrock model ID", fake.gotModelID)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(fake.gotBody, &sent); err != nil {
		t.Fatalf("invoker received non-JSON body: %v", err)
	}
	if _, present := sent["model"]; present {
		t.Error("invoker must receive the translated body (model field stripped)")
	}
	if !bytes.Equal(got, fake.invokeResp) {
		t.Errorf("InvokeModelViaBedrock() = %s, want identity of invoker response %s", got, fake.invokeResp)
	}
}

// TestInvokeModelViaBedrock_PropagatesInvokerError verifies invoker errors
// surface to the caller rather than being swallowed.
func TestInvokeModelViaBedrock_PropagatesInvokerError(t *testing.T) {
	fake := &fakeBedrockInvoker{invokeErr: errors.New("bedrock: access denied")}
	_, err := InvokeModelViaBedrock(context.Background(), fake, "m", []byte(`{}`))
	if err == nil {
		t.Fatal("expected propagated invoker error, got nil")
	}
}

// TestInvokeModelStreamViaBedrock_DelegatesToInvokeStream verifies the
// streaming path invokes InvokeStream (not Invoke) with the translated
// body, and relays whatever the invoker returns.
func TestInvokeModelStreamViaBedrock_DelegatesToInvokeStream(t *testing.T) {
	fake := &fakeBedrockInvoker{
		streamResp: "event: message_start\ndata: {}\n\n",
	}
	reqBody := []byte(`{"model":"anthropic.claude-3-5-sonnet-20241022-v2:0","stream":true,"messages":[]}`)

	rc, err := InvokeModelStreamViaBedrock(context.Background(), fake, "anthropic.claude-3-5-sonnet-20241022-v2:0", reqBody)
	if err != nil {
		t.Fatalf("InvokeModelStreamViaBedrock() error = %v", err)
	}
	defer func() { _ = rc.Close() }()

	if fake.invokeStreamCalls != 1 {
		t.Errorf("InvokeStream called %d times, want 1", fake.invokeStreamCalls)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read stream error = %v", err)
	}
	if string(got) != fake.streamResp {
		t.Errorf("stream body = %q, want %q", got, fake.streamResp)
	}
}
