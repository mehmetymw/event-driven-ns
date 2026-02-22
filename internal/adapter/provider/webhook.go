package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"

	"github.com/mehmetymw/event-driven-ns/internal/domain"
	"github.com/mehmetymw/event-driven-ns/internal/port"
	"github.com/mehmetymw/event-driven-ns/pkg/circuitbreaker"
	"github.com/mehmetymw/event-driven-ns/pkg/logger"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

type WebhookProvider struct {
	webhookURL string
	httpClient *http.Client
	breakers   map[domain.Channel]*circuitbreaker.Breaker
}

func NewWebhookProvider(webhookURL string) *WebhookProvider {
	return &WebhookProvider{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		breakers: map[domain.Channel]*circuitbreaker.Breaker{
			domain.ChannelSMS:   circuitbreaker.New("sms"),
			domain.ChannelEmail: circuitbreaker.New("email"),
			domain.ChannelPush:  circuitbreaker.New("push"),
		},
	}
}

type webhookRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

type webhookResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func (p *WebhookProvider) Send(ctx context.Context, n *domain.Notification) (*port.ProviderResponse, error) {
	breaker, ok := p.breakers[n.Channel]
	if !ok {
		breaker = circuitbreaker.New(string(n.Channel))
		p.breakers[n.Channel] = breaker
	}

	result, err := breaker.Execute(func() (any, error) {
		return p.doSend(ctx, n)
	})
	if err != nil {
		return nil, err
	}

	return result.(*port.ProviderResponse), nil
}

func (p *WebhookProvider) doSend(ctx context.Context, n *domain.Notification) (*port.ProviderResponse, error) {
	ctx, span := tracing.Tracer().Start(ctx, "webhook.send")
	defer span.End()

	span.SetAttributes(
		attribute.String("webhook.url", p.webhookURL),
		attribute.String("notification.channel", string(n.Channel)),
		attribute.String("notification.recipient", n.Recipient),
	)

	reqBody := webhookRequest{
		To:      n.Recipient,
		Channel: string(n.Channel),
		Content: n.Content,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	correlationID := logger.CorrelationIDFromContext(ctx)
	if correlationID != "" {
		req.Header.Set("X-Correlation-ID", correlationID)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, fmt.Errorf("%w: %v", domain.ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	if isTransientError(resp.StatusCode) {
		transientErr := fmt.Errorf("%w: status %d", domain.ErrProviderUnavailable, resp.StatusCode)
		tracing.RecordError(span, transientErr)
		return nil, transientErr
	}

	if resp.StatusCode >= 400 {
		permErr := fmt.Errorf("permanent provider error: status %d, body: %s", resp.StatusCode, string(respBody))
		tracing.RecordError(span, permErr)
		return nil, permErr
	}

	var webhookResp webhookResponse
	if err := json.Unmarshal(respBody, &webhookResp); err != nil {
		webhookResp = webhookResponse{
			MessageID: uuid.New().String(),
			Status:    "accepted",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	span.SetAttributes(attribute.String("webhook.message_id", webhookResp.MessageID))

	return &port.ProviderResponse{
		MessageID: webhookResp.MessageID,
		Status:    webhookResp.Status,
		Timestamp: webhookResp.Timestamp,
	}, nil
}

func isTransientError(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
