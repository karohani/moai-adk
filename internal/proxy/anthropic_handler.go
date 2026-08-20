package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
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
	bedrock        BedrockInvoker
}

// NewMessagesHandler constructs a MessagesHandler for reg/catalog. bedrock
// may be nil when no bedrock-typed group is active this session; a request
// resolving to a bedrock group with bedrock == nil returns a 500 naming the
// misconfiguration rather than a nil-pointer panic.
func NewMessagesHandler(reg *Registry, catalog *Catalog, bedrock BedrockInvoker) (*MessagesHandler, error) {
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
		bedrock:        bedrock,
	}, nil
}

func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "proxy: only POST is supported on /v1/messages", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "proxy: read request body: "+err.Error(), http.StatusBadRequest)
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
		h.serveLiteLLM(w, r, dep.Group, body)
	case GroupTypeBedrock:
		h.serveBedrock(w, r, dep.Model, body, payload.Stream)
	default:
		http.Error(w, fmt.Sprintf("proxy: group type %q is not yet wired into the daemon /v1/messages surface", group.Type), http.StatusNotImplemented)
	}
}

func (h *MessagesHandler) serveLiteLLM(w http.ResponseWriter, r *http.Request, groupName string, body []byte) {
	proxy, ok := h.litellmProxies[groupName]
	if !ok {
		http.Error(w, fmt.Sprintf("proxy: litellm group %q has no base_url configured", groupName), http.StatusInternalServerError)
		return
	}
	// Restore the body the ReadAll above consumed so the reverse proxy
	// relays it verbatim (REQ-PROXY-017: pure passthrough, no translation).
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	proxy.ServeHTTP(w, r)
}

func (h *MessagesHandler) serveBedrock(w http.ResponseWriter, r *http.Request, modelID string, body []byte, stream bool) {
	if h.bedrock == nil {
		http.Error(w, "proxy: bedrock group resolved but no BedrockInvoker is configured for this daemon", http.StatusInternalServerError)
		return
	}

	if stream {
		rc, err := InvokeModelStreamViaBedrock(r.Context(), h.bedrock, modelID, body)
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

	resp, err := InvokeModelViaBedrock(r.Context(), h.bedrock, modelID, body)
	if err != nil {
		http.Error(w, "proxy: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}
