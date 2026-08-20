package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// BedrockAnthropicVersion is the fixed anthropic_version value Bedrock's
// Anthropic-on-Bedrock InvokeModel contract requires in every request body.
const BedrockAnthropicVersion = "bedrock-2023-05-31"

// @MX:ANCHOR: [AUTO] BedrockInvoker is the seam every Bedrock-routed
// request in the daemon's /v1/messages handler flows through.
// @MX:REASON: fan_in >= 3 once M4 CLI wiring lands — the daemon handler,
// the AWS-concrete adapter, and every bedrock-group test double all depend
// on this interface's shape.

// BedrockInvoker is the minimal Bedrock runtime surface the proxy needs.
// It is deliberately narrow and independent of the concrete AWS SDK types
// so the request/response translation logic in this file is unit-testable
// without AWS credentials, network access, or SigV4 signing — the SDK
// handles signing; these tests never exercise it (see bedrock_aws.go for
// the thin concrete adapter and its documented test-coverage boundary).
type BedrockInvoker interface {
	// Invoke performs a synchronous (non-streaming) model call.
	Invoke(ctx context.Context, modelID string, body []byte) ([]byte, error)
	// InvokeStream performs a streaming model call and returns the relayed
	// response body as an io.ReadCloser the caller must Close.
	InvokeStream(ctx context.Context, modelID string, body []byte) (io.ReadCloser, error)
}

// ToBedrockInvokeModelBody rewrites an Anthropic /v1/messages request body
// into the body Bedrock's InvokeModel API expects for Anthropic-family
// models.
//
// Decision (design.md §5.4, resolved in this milestone — see progress.md
// §E.2 M2 subsection for the full field-mapping table and rationale):
// InvokeModel/InvokeModelWithResponseStream were chosen over the
// model-agnostic Converse API. The proxy's client surface IS Anthropic
// /v1/messages, and Anthropic-family Bedrock models accept the Anthropic
// Messages body almost verbatim via InvokeModel — the only differences are
// the fields this function adds/removes. Converse would instead require a
// bidirectional field remapping (renamed content-block types, camelCase
// nesting under inferenceConfig) AND full streaming-event reconstruction,
// for a model-agnosticism benefit this v1 scope does not need (v1 bedrock
// groups route Anthropic-shaped model IDs only).
func ToBedrockInvokeModelBody(anthropicBody []byte) ([]byte, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(anthropicBody, &m); err != nil {
		return nil, fmt.Errorf("proxy: parse anthropic request body: %w", err)
	}

	// model goes in the ModelId API parameter, not the body.
	delete(m, "model")
	// stream is chosen by which SDK operation is invoked (Invoke vs
	// InvokeStream), not a body field.
	delete(m, "stream")
	// anthropic_version is required by Bedrock's Anthropic-on-Bedrock
	// InvokeModel contract and has no Anthropic-native equivalent field.
	m["anthropic_version"] = BedrockAnthropicVersion

	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("proxy: marshal bedrock invoke-model body: %w", err)
	}
	return out, nil
}

// FromBedrockInvokeModelBody is the identity transform on the return path:
// Bedrock's InvokeModel response body for Anthropic-family models IS the
// Anthropic /v1/messages response shape verbatim (content blocks, usage,
// stop_reason) — no field remapping needed.
func FromBedrockInvokeModelBody(bedrockBody []byte) []byte {
	return bedrockBody
}

// InvokeModelViaBedrock performs the non-streaming Anthropic-on-Bedrock
// request/response cycle: translate the incoming Anthropic body, invoke
// modelID via inv, and identity-translate the response back.
func InvokeModelViaBedrock(ctx context.Context, inv BedrockInvoker, modelID string, anthropicBody []byte) ([]byte, error) {
	translated, err := ToBedrockInvokeModelBody(anthropicBody)
	if err != nil {
		return nil, err
	}
	resp, err := inv.Invoke(ctx, modelID, translated)
	if err != nil {
		return nil, fmt.Errorf("proxy: bedrock invoke model %q: %w", modelID, err)
	}
	return FromBedrockInvokeModelBody(resp), nil
}

// InvokeModelStreamViaBedrock performs the streaming Anthropic-on-Bedrock
// request cycle: translate the incoming Anthropic body and invoke modelID's
// streaming operation via inv, returning the relayed response stream.
//
// The stream's wire-level reconstruction (Bedrock's binary EventStream
// framing -> literal `event: <type>\ndata: <json>\n\n` SSE frames matching
// Anthropic's /v1/messages streaming contract) is the concrete AWS
// adapter's responsibility (bedrock_aws.go) — this function's contract is
// simply "relay whatever inv.InvokeStream returns", which is what makes it
// unit-testable against a fake without needing to fabricate real AWS
// EventStream bytes.
func InvokeModelStreamViaBedrock(ctx context.Context, inv BedrockInvoker, modelID string, anthropicBody []byte) (io.ReadCloser, error) {
	translated, err := ToBedrockInvokeModelBody(anthropicBody)
	if err != nil {
		return nil, err
	}
	rc, err := inv.InvokeStream(ctx, modelID, translated)
	if err != nil {
		return nil, fmt.Errorf("proxy: bedrock invoke model stream %q: %w", modelID, err)
	}
	return rc, nil
}
