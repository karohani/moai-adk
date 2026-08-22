package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

// @MX:ANCHOR: [AUTO] MessagesHandler is the daemon's single HTTP entry
// point — every routed request (litellm passthrough, bedrock adapter, and
// future codex/openai-compatible adapters) flows through ServeHTTP.
// @MX:REASON: fan_in >= 3 once M4 CLI wiring lands — StartServer(handler)
// in the CLI entrypoint, every group-type routing test, and the daemon's
// own self-termination lifecycle all depend on this handler's shape.

// anthropicMessagesRequest is the minimal shape MessagesHandler needs to
// read off an incoming request body to route it: the model identifier
// (REQ-PROXY-014/015) and whether streaming was requested. Every other
// field is relayed verbatim without being unmarshaled.
// maxRequestBodyBytes caps a single /v1/messages request body. Large
// enough for a full conversation with inline images, small enough that one
// hostile local caller cannot exhaust the shared daemon's memory.
const maxRequestBodyBytes = 32 << 20 // 32 MiB

// backendDialTimeout / backendResponseHeaderTimeout bound the phases of a
// backend call that can hang indefinitely, without bounding the streaming
// body itself.
const (
	backendDialTimeout           = 10 * time.Second
	backendResponseHeaderTimeout = 120 * time.Second
)

type anthropicMessagesRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// MessagesHandler implements the Anthropic-compatible /v1/messages HTTP
// surface (REQ-PROXY-016): it resolves the request's model field through
// the Catalog, then dispatches to the resolved group's backend adapter.
type MessagesHandler struct {
	registry       *Registry
	catalog        *Catalog
	litellmProxies map[string]*httputil.ReverseProxy // group name -> passthrough proxy
	// bedrocks is keyed by GROUP NAME. A single shared invoker (the previous
	// design) signed every bedrock request with one group's region+profile
	// regardless of which group the catalog resolved, so a registry with two
	// bedrock groups billed the wrong account and crossed data-residency
	// boundaries nondeterministically — the selection came from Go map
	// iteration order and therefore changed between daemon restarts.
	bedrocks     map[string]BedrockInvoker
	openaiClient *http.Client // shared HTTP client for openai-compatible / codex backend calls
}

// NewMessagesHandler constructs a MessagesHandler for reg/catalog. bedrocks
// maps GROUP NAME to that group's invoker and may be nil or partial when no
// bedrock-typed group is configured; a request resolving to a bedrock group
// with no entry returns a 500 naming the misconfiguration rather than a
// nil-pointer panic.
func NewMessagesHandler(reg *Registry, catalog *Catalog, bedrocks map[string]BedrockInvoker) (*MessagesHandler, error) {
	proxies := make(map[string]*httputil.ReverseProxy)
	for name, g := range reg.Groups {
		if g.Type != GroupTypeLiteLLM {
			continue
		}
		if g.BaseURL == "" {
			continue // no base_url configured yet; ServeHTTP surfaces this per-request
		}
		p, err := NewLiteLLMProxy(g.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("proxy: build litellm proxy for group %q: %w", name, err)
		}
		proxies[name] = p
	}

	return &MessagesHandler{
		registry:       reg,
		catalog:        catalog,
		litellmProxies: proxies,
		bedrocks:       bedrocks,
		openaiClient: &http.Client{
			// No Client.Timeout: it applies to the WHOLE exchange including
			// the streaming body, so any value would truncate long
			// responses. Bound connection setup instead, which is where a
			// misconfigured or unreachable base_url actually hangs.
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: backendDialTimeout}).DialContext,
				TLSHandshakeTimeout:   backendDialTimeout,
				ResponseHeaderTimeout: backendResponseHeaderTimeout,
			},
		},
	}, nil
}

func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "proxy: only POST is supported on /v1/messages", http.StatusMethodNotAllowed)
		return
	}

	// Bound the buffered body. Every request is fully materialized in memory
	// before routing, and the daemon is shared by every project on the
	// machine, so an unbounded ReadAll let one local request consume
	// arbitrary memory for all of them.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err != nil {
		http.Error(w, "proxy: read request body: "+err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	_ = r.Body.Close()

	var payload anthropicMessagesRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "proxy: invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if payload.Model == "" {
		http.Error(w, "proxy: request body must carry a non-empty \"model\" field", http.StatusBadRequest)
		return
	}

	dep, err := h.catalog.Resolve(payload.Model)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadRequest)
		return
	}

	group, ok := h.registry.Groups[dep.Group]
	if !ok {
		http.Error(w, fmt.Sprintf("proxy: resolved group %q not found in registry", dep.Group), http.StatusInternalServerError)
		return
	}

	switch group.Type {
	case GroupTypeLiteLLM:
		h.serveLiteLLM(w, r, dep.Group, body, dep.Model)
	case GroupTypeBedrock:
		h.serveBedrock(w, r, dep.Group, dep.Model, body, payload.Stream)
	case GroupTypeOpenAICompatible:
		if group.BaseURL == "" {
			http.Error(w, fmt.Sprintf("proxy: openai-compatible group %q has no base_url configured", dep.Group), http.StatusInternalServerError)
			return
		}
		serveOpenAICompatible(w, r, h.openaiClient, group.BaseURL, body, dep.Model, payload.Stream)
	case GroupTypeCodex:
		serveCodex(w, r, h.openaiClient, group, body, dep.Model, payload.Stream)
	default:
		// GroupTypeCopilot (and any future unimplemented type) lands here —
		// copilot is explicitly OUT of v1 scope (plan.md M3 item 8,
		// spec.md §H); the type enum entry exists from M1 for forward
		// compatibility, but no backend adapter is implemented in v1.
		http.Error(w, fmt.Sprintf("proxy: group type %q is not yet wired into the daemon /v1/messages surface", group.Type), http.StatusNotImplemented)
	}
}

func (h *MessagesHandler) serveLiteLLM(w http.ResponseWriter, r *http.Request, groupName string, body []byte, model string) {
	proxy, ok := h.litellmProxies[groupName]
	if !ok {
		http.Error(w, fmt.Sprintf("proxy: litellm group %q has no base_url configured", groupName), http.StatusInternalServerError)
		return
	}

	// REQ-PROXY-017's "no translation" governs the request SHAPE — litellm
	// speaks Anthropic natively, so no field mapping is performed. It cannot
	// extend to the model identifier: the client-facing value is either an
	// alias this proxy defined (`opus`) or a `<group>/<model>` reference this
	// proxy's own naming scheme invented (`llm-a/gpt-4o`). Relaying either
	// verbatim forwards a name the backend has never heard of, so the one
	// substitution the catalog exists to make must still be applied.
	relayBody, err := replaceModelField(body, model)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadRequest)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(relayBody))
	r.ContentLength = int64(len(relayBody))
	proxy.ServeHTTP(w, r)
}

// replaceModelField rewrites ONLY the top-level "model" field of an
// Anthropic request body, leaving every other field byte-for-byte as the
// client sent it. Round-tripping through a generic map would reorder keys
// and normalize numbers; keeping the edit surgical preserves the passthrough
// guarantee everywhere it actually applies.
func replaceModelField(body []byte, model string) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse request body: %w", err)
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("encode resolved model: %w", err)
	}
	raw["model"] = encoded
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-encode request body: %w", err)
	}
	return out, nil
}

func (h *MessagesHandler) serveBedrock(w http.ResponseWriter, r *http.Request, groupName, modelID string, body []byte, stream bool) {
	// Look the invoker up by the RESOLVED group so the request is signed
	// with that group's own region and profile.
	invoker, ok := h.bedrocks[groupName]
	if !ok || invoker == nil {
		http.Error(w, fmt.Sprintf("proxy: bedrock group %q resolved but no BedrockInvoker is configured for it in this daemon", groupName), http.StatusInternalServerError)
		return
	}

	if stream {
		rc, err := InvokeModelStreamViaBedrock(r.Context(), invoker, modelID, body)
		if err != nil {
			http.Error(w, "proxy: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = rc.Close() }()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, rerr := rc.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if rerr != nil {
				return
			}
		}
	}

	resp, err := InvokeModelViaBedrock(r.Context(), invoker, modelID, body)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}
