package handlers_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexusdeploy/backend/services/api-gateway/handlers"
	"github.com/stretchr/testify/require"
)

type recordingProcessor struct {
	called bool
	event  handlers.WebhookEvent
	err    error
}

func (r *recordingProcessor) ProcessWebhook(req *http.Request, event handlers.WebhookEvent) error {
	r.called = true
	r.event = event
	return r.err
}

func TestWebhookHandler_InvalidMethod(t *testing.T) {
	handler := handlers.NewWebhookHandler("secret", nil)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/github", nil)
	rec := httptest.NewRecorder()

	handler.HandleGitHubWebhook(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestWebhookHandler_MissingSignature(t *testing.T) {
	handler := handlers.NewWebhookHandler("secret", nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewBufferString("{}"))
	rec := httptest.NewRecorder()

	handler.HandleGitHubWebhook(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	handler := handlers.NewWebhookHandler("secret", nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewBufferString("{}"))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	rec := httptest.NewRecorder()

	handler.HandleGitHubWebhook(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebhookHandler_MissingEventHeader(t *testing.T) {
	secret := "webhook-secret"
	handler := handlers.NewWebhookHandler(secret, nil)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", makeSignature(secret, payload))

	rec := httptest.NewRecorder()
	handler.HandleGitHubWebhook(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookHandler_Success(t *testing.T) {
	secret := "webhook-secret"
	processor := &recordingProcessor{}
	handler := handlers.NewWebhookHandler(secret, processor)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", makeSignature(secret, payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-123")

	rec := httptest.NewRecorder()
	handler.HandleGitHubWebhook(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	require.True(t, processor.called)
	require.Equal(t, "push", processor.event.Event)
	require.Equal(t, "delivery-123", processor.event.DeliveryID)
	require.Equal(t, payload, processor.event.Payload)
}

func makeSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
