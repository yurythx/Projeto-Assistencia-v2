package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
)

// Deliverer faz o POST do webhook pelo cliente HTTP anti-SSRF.
type Deliverer struct {
	client *http.Client
	policy netguard.Policy
}

// NewDeliverer cria o entregador.
func NewDeliverer(timeout time.Duration, policy netguard.Policy) *Deliverer {
	return &Deliverer{client: netguard.NewClient(timeout, policy), policy: policy}
}

// Policy devolve a política anti-SSRF em uso (validação no cadastro).
func (d *Deliverer) Policy() netguard.Policy { return d.policy }

// Body monta o corpo por tipo de destino. n8n e webhook genérico recebem
// o envelope; Zabbix e Grafana, um formato de alerta simples.
func Body(kind string, eventID, eventType string, payload json.RawMessage, at time.Time) ([]byte, error) {
	switch kind {
	case "zabbix", "grafana":
		return json.Marshal(map[string]any{
			"title":       "Nexus: " + eventType,
			"message":     string(payload),
			"event_id":    eventID,
			"event_type":  eventType,
			"state":       "info",
			"occurred_at": at.UTC().Format(time.RFC3339),
		})
	default:
		return json.Marshal(map[string]any{
			"id": eventID, "type": eventType, "occurred_at": at.UTC().Format(time.RFC3339), "data": payload,
		})
	}
}

// Sign calcula a assinatura HMAC-SHA256 "t=<unix>,v1=<hex>" sobre
// "<timestamp>.<corpo>" — o receptor valida origem e frescor (anti-replay).
func Sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

// Deliver envia a entrega. Devolve o status HTTP (0 se não houve resposta).
func (d *Deliverer) Deliver(ctx context.Context, t domain.Target, del domain.Delivery) (int, error) {
	if _, err := netguard.ValidateURL(t.URL, d.policy); err != nil {
		return 0, err
	}
	body, err := Body(t.Kind, del.EventID.String(), del.EventType, del.Payload, del.CreatedAt)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Nexus-Egress/1.0")
	req.Header.Set("X-Nexus-Event", del.EventType)
	req.Header.Set("X-Nexus-Delivery", del.ID.String())
	req.Header.Set("X-Idempotency-Key", del.EventID.String())
	if t.Secret != "" {
		req.Header.Set("X-Nexus-Signature", Sign(t.Secret, time.Now().Unix(), body))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("destino respondeu HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
