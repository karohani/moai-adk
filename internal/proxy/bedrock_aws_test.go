package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// TestNewAWSBedrockInvoker_ConstructsFromRegionAndProfile verifies
// construction succeeds without requiring live AWS credentials — the SDK's
// LoadDefaultConfig resolves configuration lazily; actual credential
// validation and SigV4 signing happen only on the network call, which this
// test does not make (per the documented residual-risk boundary: signing
// itself is never unit tested).
func TestNewAWSBedrockInvoker_ConstructsFromRegionAndProfile(t *testing.T) {
	inv, err := NewAWSBedrockInvoker(context.Background(), "us-east-1", "")
	if err != nil {
		t.Fatalf("NewAWSBedrockInvoker() error = %v", err)
	}
	if inv == nil || inv.client == nil {
		t.Fatal("NewAWSBedrockInvoker() returned a nil client")
	}
}

// newTestAWSBedrockInvoker builds an AWSBedrockInvoker whose underlying SDK
// client is pointed at a local httptest server via BaseEndpoint override,
// using static (fake) credentials so no real AWS network call or IAM
// resolution occurs. This exercises the SDK's actual request-building,
// signing, and response-parsing machinery against a hermetic backend —
// distinct from fakeBedrockInvoker (used elsewhere), which bypasses the SDK
// entirely.
func newTestAWSBedrockInvoker(t *testing.T, endpoint string) *AWSBedrockInvoker {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secrettest", "")),
	)
	if err != nil {
		t.Fatalf("LoadDefaultConfig() error = %v", err)
	}
	client := bedrockruntime.NewFromConfig(cfg, func(o *bedrockruntime.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	return &AWSBedrockInvoker{client: client}
}

// TestAWSBedrockInvoker_InvokeSendsRequestAndReturnsBody exercises the real
// SDK's InvokeModel request/response cycle against a local httptest server
// standing in for the bedrock-runtime endpoint.
func TestAWSBedrockInvoker_InvokeSendsRequestAndReturnsBody(t *testing.T) {
	wantBody := `{"id":"msg_1","content":[{"type":"text","text":"hi"}]}`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if len(b) == 0 {
			t.Error("bedrock endpoint received an empty request body")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(wantBody))
	}))
	defer backend.Close()

	inv := newTestAWSBedrockInvoker(t, backend.URL)
	got, err := inv.Invoke(context.Background(), "anthropic.claude-3-5-sonnet-20241022-v2:0", []byte(`{"messages":[]}`))
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if string(got) != wantBody {
		t.Errorf("Invoke() = %s, want %s", got, wantBody)
	}
}

// TestAWSBedrockInvoker_InvokeErrorPropagates verifies a backend error
// status surfaces as a Go error rather than being swallowed.
func TestAWSBedrockInvoker_InvokeErrorPropagates(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"access denied"}`))
	}))
	defer backend.Close()

	inv := newTestAWSBedrockInvoker(t, backend.URL)
	_, err := inv.Invoke(context.Background(), "anthropic.claude-3-5-sonnet-20241022-v2:0", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 403 backend response, got nil")
	}
}

// TestAWSBedrockInvoker_InvokeStreamErrorPropagates verifies a backend
// error status on the streaming operation surfaces as a Go error. Full
// happy-path event-stream framing (AWS's vnd.amazon.eventstream wire
// format) is NOT exercised here — reconstructing valid event-stream binary
// frames by hand is out of proportion to what this milestone's scope
// affords, and is recorded as a residual risk in progress.md §E.2 rather
// than silently assumed correct.
func TestAWSBedrockInvoker_InvokeStreamErrorPropagates(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"access denied"}`))
	}))
	defer backend.Close()

	inv := newTestAWSBedrockInvoker(t, backend.URL)
	_, err := inv.InvokeStream(context.Background(), "anthropic.claude-3-5-sonnet-20241022-v2:0", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 403 backend response, got nil")
	}
}
