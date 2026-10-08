package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/egress/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
)

// O webhook sai assinado (HMAC com timestamp), com cabeçalhos de rastreio
// e idempotência, no formato do tipo de destino.
func TestDeliverer(t *testing.T) {
	var got *http.Request
	var body []byte
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.WriteHeader(status)
	}))
	defer srv.Close()
	d := NewDeliverer(5*time.Second, netguard.Policy{AllowPrivate: true, AllowHTTP: true})
	if !d.Policy().AllowPrivate {
		t.Fatal("política exposta para a validação do cadastro")
	}
	del := domain.Delivery{ID: uuid.New(), EventID: uuid.New(), EventType: "blog.post.published", Payload: json.RawMessage(`{"id":"1"}`), CreatedAt: time.Now()}
	target := domain.Target{Kind: "n8n", URL: srv.URL + "/hook", Secret: "s3cr3t"}

	code, err := d.Deliver(context.Background(), target, del)
	if err != nil || code != http.StatusOK {
		t.Fatalf("entrega: %d %v", code, err)
	}
	sig := got.Header.Get("X-Nexus-Signature")
	if got.Header.Get("X-Nexus-Event") != del.EventType || got.Header.Get("X-Idempotency-Key") != del.EventID.String() ||
		got.Header.Get("X-Nexus-Delivery") != del.ID.String() || !strings.HasPrefix(sig, "t=") {
		t.Fatalf("cabeçalhos do webhook: %v", got.Header)
	}
	var ts int64
	var mac string
	if _, err := fmt.Sscanf(sig, "t=%d,v1=%s", &ts, &mac); err != nil || Sign("s3cr3t", ts, body) != sig {
		t.Fatalf("o receptor valida a assinatura com o segredo: %q", sig)
	}
	if !strings.Contains(string(body), `"type":"blog.post.published"`) {
		t.Fatalf("n8n/webhook recebem o envelope: %s", body)
	}

	// Zabbix/Grafana recebem alerta; sem segredo, sem assinatura.
	if _, err := d.Deliver(context.Background(), domain.Target{Kind: "zabbix", URL: srv.URL}, del); err != nil {
		t.Fatal(err)
	}
	if got.Header.Get("X-Nexus-Signature") != "" || !strings.Contains(string(body), `"title":"Nexus: blog.post.published"`) {
		t.Fatalf("alerta do Zabbix sem assinatura: %v %s", got.Header, body)
	}
	status = http.StatusBadGateway
	if code, err := d.Deliver(context.Background(), target, del); err == nil || code != http.StatusBadGateway {
		t.Fatalf("resposta fora de 2xx é falha com o status: %d %v", code, err)
	}
	// Política padrão (produção) bloqueia loopback e http.
	strict := NewDeliverer(time.Second, netguard.Policy{})
	if _, err := strict.Deliver(context.Background(), target, del); !errors.Is(err, netguard.ErrBlockedDestination) {
		t.Fatalf("destino interno bloqueado pela política anti-SSRF: %v", err)
	}
	srv.Close()
	if _, err := d.Deliver(context.Background(), target, del); err == nil {
		t.Fatal("destino fora do ar é falha")
	}
	var noCtx context.Context // requisição sem contexto não é montada
	if _, err := d.Deliver(noCtx, target, del); err == nil {
		t.Fatal("requisição inválida não é enviada")
	}
	if _, err := Body("webhook", "id", "t", json.RawMessage(`{`), time.Now()); err == nil {
		t.Fatal("payload inválido não vira corpo")
	}
	if _, err := d.Deliver(context.Background(), target, domain.Delivery{Payload: json.RawMessage(`{`)}); err == nil {
		t.Fatal("entrega com payload inválido falha")
	}
}

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	calls := map[string]func(db database.DBTX) error{
		"Targets":         func(db database.DBTX) error { _, err := r.Targets(ctx, db, true); return err },
		"Target":          func(db database.DBTX) error { _, err := r.Target(ctx, db, id); return err },
		"SaveTarget":      func(db database.DBTX) error { _, err := r.SaveTarget(ctx, db, domain.Target{ID: id}, nil); return err },
		"DeleteTarget":    func(db database.DBTX) error { return r.DeleteTarget(ctx, db, id) },
		"EncryptedSecret": func(db database.DBTX) error { _, err := r.EncryptedSecret(ctx, db, id); return err },
		"EnqueueDelivery": func(db database.DBTX) error { return r.EnqueueDelivery(ctx, db, domain.Delivery{}) },
		"ClaimDue":        func(db database.DBTX) error { _, err := r.ClaimDue(ctx, db, 10); return err },
		"MarkDelivered":   func(db database.DBTX) error { return r.MarkDelivered(ctx, db, id, 200) },
		"MarkFailed":      func(db database.DBTX) error { return r.MarkFailed(ctx, db, id, 1, nil, "x", time.Now(), true) },
		"Deliveries": func(db database.DBTX) error {
			_, _, err := r.Deliveries(ctx, db, nil, "", pagination.New(1, 10, 10))
			return err
		},
		"Redeliver": func(db database.DBTX) error { return r.Redeliver(ctx, db, id) },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"Targets", "ClaimDue"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
		if err := calls[name](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com erro durante a leitura: %v", name, err)
		}
	}
	for _, name := range []string{"DeleteTarget", "Redeliver"} {
		if err := calls[name](dbtest.ScanFail{}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s sem linha afetada: %v", name, err)
		}
	}
	if err := calls["SaveTarget"](&dbtest.Seq{Execs: []dbtest.ExecResult{dbtest.OK}}); err == nil {
		t.Error("gravou mas não releu o destino")
	}
	page := &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{{Err: dbtest.ErrInjected}}}
	if _, _, err := r.Deliveries(ctx, page, nil, "", pagination.New(1, 10, 10)); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("contagem ok, página falha: %v", err)
	}
	bad := &dbtest.Seq{Row: countRow{}, Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}}
	if _, _, err := r.Deliveries(ctx, bad, nil, "", pagination.New(1, 10, 10)); err == nil {
		t.Error("linha ilegível na página de entregas")
	}
}

type countRow struct{}

func (countRow) Scan(dest ...any) error {
	*(dest[0].(*int64)) = 0
	return nil
}
