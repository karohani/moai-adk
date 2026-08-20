package proxy

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// AWSBedrockInvoker is the thin concrete BedrockInvoker implementation
// backed by the real AWS SDK. It deliberately contains as little logic as
// possible: SigV4 request signing and AWS transport are entirely the SDK's
// responsibility and are NEVER exercised by this package's unit tests (no
// AWS credentials or network access are available in CI) — the
// request/response translation this adapter delegates to (Invoke /
// InvokeStream call ToBedrockInvokeModelBody indirectly via
// InvokeModelViaBedrock / InvokeModelStreamViaBedrock, both fully unit
// tested against a fake in bedrock_test.go) carries the tested logic.
//
// Residual risk (documented, not silently assumed): live-AWS verification
// of this adapter — a real Converse-family Anthropic-on-Bedrock round trip,
// including the true EventStream-to-SSE reconstruction below — has not been
// exercised in this environment. See progress.md §E.2 M2 Gaps.
type AWSBedrockInvoker struct {
	client *bedrockruntime.Client
}

// NewAWSBedrockInvoker constructs an AWSBedrockInvoker using the AWS SDK's
// default credential chain and region resolution (profile name / region are
// registry-configured references only — REQ-PROXY-024 credential-reference
// boundary; actual credentials are never read or stored by moai proxy
// itself). profile may be empty to use the SDK's default profile selection.
func NewAWSBedrockInvoker(ctx context.Context, region, profile string) (*AWSBedrockInvoker, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("proxy: load AWS config for bedrock group: %w", err)
	}

	return &AWSBedrockInvoker{client: bedrockruntime.NewFromConfig(cfg)}, nil
}

// Invoke implements BedrockInvoker.Invoke via the SDK's InvokeModel
// operation.
func (a *AWSBedrockInvoker) Invoke(ctx context.Context, modelID string, body []byte) ([]byte, error) {
	out, err := a.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("proxy: bedrock InvokeModel %q: %w", modelID, err)
	}
	return out.Body, nil
}

// InvokeStream implements BedrockInvoker.InvokeStream via the SDK's
// InvokeModelWithResponseStream operation, re-wrapping each EventStream
// chunk's bytes (which are themselves Anthropic-shaped SSE event JSON) into
// literal `event: <type>\ndata: <json>\n\n` frames matching what an
// Anthropic-native /v1/messages streaming response looks like.
func (a *AWSBedrockInvoker) InvokeStream(ctx context.Context, modelID string, body []byte) (io.ReadCloser, error) {
	out, err := a.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("proxy: bedrock InvokeModelWithResponseStream %q: %w", modelID, err)
	}
	return newBedrockEventStreamReader(out.GetStream()), nil
}

// bedrockEventStreamReader adapts the AWS SDK's typed EventStream reader
// (a channel of ResponseStream events, each carrying a PayloadPart chunk)
// into an io.ReadCloser of literal Anthropic-shaped SSE bytes.
type bedrockEventStreamReader struct {
	stream *bedrockruntime.InvokeModelWithResponseStreamEventStream
	buf    []byte
}

func newBedrockEventStreamReader(stream *bedrockruntime.InvokeModelWithResponseStreamEventStream) *bedrockEventStreamReader {
	return &bedrockEventStreamReader{stream: stream}
}

func (r *bedrockEventStreamReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		event, ok := <-r.stream.Events()
		if !ok {
			if err := r.stream.Err(); err != nil {
				return 0, fmt.Errorf("proxy: bedrock event stream: %w", err)
			}
			return 0, io.EOF
		}
		if chunk, ok := event.(*types.ResponseStreamMemberChunk); ok {
			// The chunk's Bytes field is itself Anthropic-shaped SSE event
			// JSON (message_start / content_block_delta / message_stop /
			// etc. per AWS's Anthropic-on-Bedrock streaming contract).
			// Wrapping it as a literal SSE frame matches what an
			// Anthropic-native streaming /v1/messages response emits.
			r.buf = append([]byte("data: "), append(chunk.Value.Bytes, []byte("\n\n")...)...)
		}
		// Non-chunk events (unknown union members) are skipped; the loop
		// continues to the next channel receive.
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *bedrockEventStreamReader) Close() error {
	return r.stream.Close()
}
