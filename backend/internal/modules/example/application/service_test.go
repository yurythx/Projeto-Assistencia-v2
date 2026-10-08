package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"

	"github.com/yurythx/projeto-nexus/internal/modules/example/application"
	"github.com/yurythx/projeto-nexus/internal/modules/example/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/example/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// Testes de integração (Postgres real) — este módulo era o único, entre
// os quatro registrados em internal/app/modules.go, sem nenhum teste
// (domain/application/infrastructure/transport). Achado de auditoria:
// rodar CreateItem contra um banco real (em vez de um repositório falso
// em memória, como users/application faz) foi exatamente o que expôs que
// a tabela example_items nunca tinha sido criada em migration nenhuma —
// ver migrations/000003_example_items.sql.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping live example module test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateItem_PersistsItemAndWritesOutboxEventInSameTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewPostgresRepository()
	writer := outbox.NewWriter("nexus.example")
	svc := application.NewService(pool, repo, writer, testLogger())

	item, err := svc.CreateItem(ctx, "Item de teste", "descrição de teste")
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM example_items WHERE id = $1`, item.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox_events WHERE aggregate_id = $1`, item.ID.String())
		// audit_logs é append-only (trigger da migration 000001) — não dá
		// para limpar a linha de auditoria; ela fica no banco de teste.
	})

	// O item precisa estar de fato gravado em example_items (não só em
	// memória) — GetItem lê direto do banco.
	stored, err := svc.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItem depois de CreateItem: %v", err)
	}
	if stored.Title != "Item de teste" {
		t.Errorf("Title = %q, want %q", stored.Title, "Item de teste")
	}
	if stored.Status != "ACTIVE" {
		t.Errorf("Status = %q, want ACTIVE", stored.Status)
	}

	// E um evento "example.item.created" precisa ter sido gravado em
	// outbox_events NA MESMA operação — a prova de que CreateItem segue
	// o padrão Transactional Outbox, não só grava o item.
	var eventType, aggregateType, status string
	var payload []byte
	err = pool.QueryRow(ctx,
		`SELECT event_type, aggregate_type, status, payload FROM outbox_events WHERE aggregate_id = $1`,
		item.ID.String(),
	).Scan(&eventType, &aggregateType, &status, &payload)
	if err != nil {
		t.Fatalf("query outbox_events: %v", err)
	}
	if eventType != "example.item.created" {
		t.Errorf("event_type = %q, want example.item.created", eventType)
	}
	if aggregateType != "example_item" {
		t.Errorf("aggregate_type = %q, want example_item", aggregateType)
	}
	if status != "pending" {
		t.Errorf("status = %q, want pending (o Publisher ainda não rodou)", status)
	}

	var envelope struct {
		Payload struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if envelope.Payload.ID != item.ID.String() || envelope.Payload.Title != "Item de teste" {
		t.Errorf("payload = %+v, want id=%s title=Item de teste", envelope.Payload, item.ID)
	}

	// E uma linha de auditoria "example.item.created" precisa ter sido
	// gravada NA MESMA transação (gap G-06): o blueprint que os novos
	// módulos copiam tem de mostrar mutação-com-trilha, não só
	// mutação-com-outbox.
	var auditAction, auditResource string
	err = pool.QueryRow(ctx,
		`SELECT action, resource_type FROM audit_logs WHERE resource_id = $1 AND action = 'example.item.created'`,
		item.ID.String(),
	).Scan(&auditAction, &auditResource)
	if err != nil {
		t.Fatalf("query audit_logs: %v (CreateItem deveria ter gravado a trilha na mesma tx)", err)
	}
	if auditResource != "example_item" {
		t.Errorf("audit resource_type = %q, want example_item", auditResource)
	}
	if auditAction != "example.item.created" {
		t.Errorf("audit action = %q, want example.item.created", auditAction)
	}
}

func TestCreateItem_RejectsEmptyTitleWithoutTouchingTheDatabase(t *testing.T) {
	pool := testPool(t)
	repo := infrastructure.NewPostgresRepository()
	writer := outbox.NewWriter("nexus.example")
	svc := application.NewService(pool, repo, writer, testLogger())

	if _, err := svc.CreateItem(context.Background(), "", "sem título"); err == nil {
		t.Fatal("expected an error for an empty title")
	}
}

func TestCreateItem_TrimsAndValidatesTitle(t *testing.T) {
	pool := testPool(t)
	svc := application.NewService(pool, infrastructure.NewPostgresRepository(), outbox.NewWriter("nexus.example"), testLogger())
	ctx := context.Background()
	for _, bad := range []string{"   ", strings.Repeat("á", 201)} {
		if _, err := svc.CreateItem(ctx, bad, ""); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("título %q deveria ser recusado pelo domínio: %v", bad[:3], err)
		}
	}
	item, err := svc.CreateItem(ctx, "  Aparado  ", "  desc  ")
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Aparado" || item.Description != "desc" || item.Status != domain.StatusActive {
		t.Fatalf("título/descrição deveriam ser aparados: %+v", item)
	}
	if _, err := svc.CreateItem(ctx, strings.Repeat("á", 200), ""); err != nil {
		t.Fatalf("200 caracteres (multibyte) cabem: %v", err)
	}
	if _, err := svc.GetItem(ctx, uuid.New()); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("item inexistente: %v", err)
	}
	if err := application.MapError(domain.ErrExampleNotFound); !strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("MapError(not found) = %v", err)
	}
	items, total, err := svc.ListItems(ctx, pagination.New(1, 1, 100))
	if err != nil || len(items) != 1 || total < 3 {
		t.Fatalf("página de 1 item: %d/%d %v", len(items), total, err)
	}
	empty, _, err := svc.ListItems(ctx, pagination.New(1_000_000, 100, 100))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("página além do fim deve ser [] (não nil): %v %v", empty, err)
	}
}
