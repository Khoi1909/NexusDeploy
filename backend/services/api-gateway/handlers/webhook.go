package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
)

const (
	headerSignature = "X-Hub-Signature-256"
	headerEvent     = "X-GitHub-Event"
	headerDelivery  = "X-GitHub-Delivery"
)

// WebhookEvent chứa thông tin webhook đã xác thực
type WebhookEvent struct {
	Event      string
	DeliveryID string
	Payload    []byte
}

// WebhookProcessor xử lý webhook sau khi được xác thực
type WebhookProcessor interface {
	ProcessWebhook(r *http.Request, event WebhookEvent) error
}

// WebhookProcessorFunc là helper để dùng function làm processor
type WebhookProcessorFunc func(r *http.Request, event WebhookEvent) error

// ProcessWebhook implement interface
func (f WebhookProcessorFunc) ProcessWebhook(r *http.Request, event WebhookEvent) error {
	if f == nil {
		return nil
	}
	return f(r, event)
}

// WebhookHandler xử lý GitHub webhook với signature validation
type WebhookHandler struct {
	Secret    string
	Processor WebhookProcessor
}

// NewWebhookHandler tạo handler
func NewWebhookHandler(secret string, processor WebhookProcessor) *WebhookHandler {
	return &WebhookHandler{
		Secret:    secret,
		Processor: processor,
	}
}

// HandleGitHubWebhook xử lý POST /webhooks/github
func (h *WebhookHandler) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusMethodNotAllowed, "Method Not Allowed", "METHOD_NOT_ALLOWED", "")
		return
	}

	if h.Secret == "" {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusInternalServerError, "Internal Server Error", "INTERNAL_ERROR", "Webhook secret is not configured")
		return
	}

	signature := r.Header.Get(headerSignature)
	if signature == "" {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusUnauthorized, "Unauthorized", "INVALID_SIGNATURE", "Missing signature header")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusBadRequest, "Bad Request", "INVALID_PAYLOAD", err.Error())
		return
	}
	defer r.Body.Close()

	if !validateSignature(signature, payload, []byte(h.Secret)) {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusUnauthorized, "Unauthorized", "INVALID_SIGNATURE", "Signature mismatch")
		return
	}

	eventName := r.Header.Get(headerEvent)
	if eventName == "" {
		commonmw.WriteErrorResponse(w, r.Context(), http.StatusBadRequest, "Bad Request", "MISSING_EVENT", "Missing X-GitHub-Event header")
		return
	}

	event := WebhookEvent{
		Event:      eventName,
		DeliveryID: r.Header.Get(headerDelivery),
		Payload:    payload,
	}

	if h.Processor != nil {
		if err := h.Processor.ProcessWebhook(r, event); err != nil {
			commonmw.WriteErrorResponse(w, r.Context(), http.StatusInternalServerError, "Internal Server Error", "PROCESSING_ERROR", err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func validateSignature(signature string, payload, secret []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}

	signature = strings.TrimPrefix(signature, prefix)
	expectedMAC := hmac.New(sha256.New, secret)
	expectedMAC.Write(payload)
	calculated := expectedMAC.Sum(nil)

	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	return hmac.Equal(calculated, sigBytes)
}
