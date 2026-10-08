package infrastructure

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/signum/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/passwords"
)

func TestRepositoryPropagatesDatabaseErrors(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	id := uuid.New()
	env := domain.Envelope{ID: id, Signers: []domain.Signer{{UserID: uuid.New()}}}
	calls := map[string]func(db database.DBTX) error{
		"Insert":             func(db database.DBTX) error { return r.Insert(ctx, db, env) },
		"UnavailableSigners": func(db database.DBTX) error { _, err := r.UnavailableSigners(ctx, db, []uuid.UUID{id}); return err },
		"Get":                func(db database.DBTX) error { _, err := r.Get(ctx, db, id, false); return err },
		"GetForUpdate":       func(db database.DBTX) error { _, err := r.Get(ctx, db, id, true); return err },
		"List":               func(db database.DBTX) error { _, err := r.List(ctx, db, domain.Filter{}, 10); return err },
		"SetEnvelopeStatus":  func(db database.DBTX) error { return r.SetEnvelopeStatus(ctx, db, id, domain.StatusCancelled) },
		"UpdateSigner":       func(db database.DBTX) error { return r.UpdateSigner(ctx, db, domain.Signer{ID: id}) },
		"InsertChallenge":    func(db database.DBTX) error { return r.InsertChallenge(ctx, db, domain.Challenge{ID: id}) },
		"ConsumeChallenge":   func(db database.DBTX) error { return r.ConsumeChallenge(ctx, db, id, id, id, "x") },
	}
	for name, call := range calls {
		if err := call(dbtest.Fail{}); !errors.Is(err, dbtest.ErrInjected) {
			t.Errorf("%s com o banco fora: %v", name, err)
		}
	}
	for _, name := range []string{"UnavailableSigners", "Get", "List"} {
		if err := calls[name](dbtest.ScanFail{}); err == nil {
			t.Errorf("%s com linha ilegível deveria falhar", name)
		}
	}
	if err := calls["UnavailableSigners"](dbtest.RowsErr{}); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("erro no meio da leitura: %v", err)
	}
	// Desafio que não casa (usado, expirado, de outro) não é consumido.
	if err := calls["ConsumeChallenge"](dbtest.ScanFail{}); !errors.Is(err, domain.ErrChallenge) {
		t.Errorf("desafio sem linha afetada: %v", err)
	}
	// Envelope existe mas os signatários falham ao carregar.
	if _, err := r.Get(ctx, headerOK{}, id, false); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("falha ao carregar signatários: %v", err)
	}
	// Na listagem, a falha nos signatários de um envelope sobe.
	if _, err := r.List(ctx, listOneThenFail{}, domain.Filter{}, 10); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("falha nos signatários da listagem: %v", err)
	}
	if err := r.Insert(ctx, &firstExecOK{}, env); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("falha ao gravar o signatário depois do envelope: %v", err)
	}
	if _, err := r.Get(ctx, signersScanFail{}, id, false); err == nil {
		t.Error("signatário ilegível deveria falhar")
	}
}

// firstExecOK aceita o primeiro Exec (o envelope) e falha nos seguintes.
type firstExecOK struct {
	dbtest.Fail
	n int
}

func (f *firstExecOK) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.n++
	if f.n == 1 {
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}
	return f.Fail.Exec(ctx, sql, args...)
}

// headerOK devolve o envelope e falha na consulta dos signatários.
type headerOK struct{ dbtest.Fail }

func (headerOK) QueryRow(context.Context, string, ...any) pgx.Row { return envelopeRow{} }

// signersScanFail devolve o envelope e signatários ilegíveis.
type signersScanFail struct{ dbtest.ScanFail }

func (signersScanFail) QueryRow(context.Context, string, ...any) pgx.Row { return envelopeRow{} }

// listOneThenFail: a lista traz um envelope; a busca dos signatários falha.
type listOneThenFail struct{ dbtest.Fail }

func (listOneThenFail) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM signum_envelopes") {
		return &envelopeRows{}, nil
	}
	return nil, dbtest.ErrInjected
}

type envelopeRow struct{}

func (envelopeRow) Scan(dest ...any) error {
	*(dest[0].(*uuid.UUID)) = uuid.New()
	return nil
}

type envelopeRows struct{ done bool }

func (r *envelopeRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *envelopeRows) Scan(dest ...any) error        { return envelopeRow{}.Scan(dest...) }
func (r *envelopeRows) Close()                        {}
func (r *envelopeRows) Err() error                    { return nil }
func (r *envelopeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *envelopeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *envelopeRows) Values() ([]any, error) { return nil, nil }
func (r *envelopeRows) RawValues() [][]byte    { return nil }
func (r *envelopeRows) Conn() *pgx.Conn        { return nil }

// Reautenticação da conta local: senha certa, errada, vazia, conta sem
// senha e conta desativada.
func TestLocalReauthentication(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	hash, err := passwords.Hash("Senha-Forte-123!")
	if err != nil {
		t.Fatal(err)
	}
	var local, inativo uuid.UUID
	for _, row := range []struct {
		id     *uuid.UUID
		active bool
	}{{&local, true}, {&inativo, false}} {
		name := "r_" + uuid.NewString()[:10]
		if err := pool.QueryRow(ctx, `INSERT INTO users (username, email, password_hash, active) VALUES ($1, $1 || '@t', $2, $3) RETURNING id`,
			name, hash, row.active).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	federado := dbtest.User(t, pool) // sem password_hash
	r := NewReauthenticator(pool, func(context.Context) (string, string, string) { return "", "", "" })

	if m, err := r.Reauthenticate(ctx, local, "x", false, "Senha-Forte-123!"); err != nil || m != "password_reauth:local" {
		t.Fatalf("senha certa: %q %v", m, err)
	}
	for name, c := range map[string]struct {
		id  uuid.UUID
		pwd string
	}{
		"senha errada": {local, "errada"}, "senha vazia": {local, ""}, "conta desativada": {inativo, "Senha-Forte-123!"},
		"conta sem senha local": {federado, "qualquer"}, "conta inexistente": {uuid.New(), "qualquer"},
	} {
		if _, err := r.Reauthenticate(ctx, c.id, "x", false, c.pwd); !errors.Is(err, domain.ErrReauth) {
			t.Errorf("%s: esperado ErrReauth, veio %v", name, err)
		}
	}
}

// Reautenticação federada: o Signum confere a senha no Keycloak (que
// valida no AD) com o grant password.
func TestFederatedReauthentication(t *testing.T) {
	var gotForm map[string]string
	status := http.StatusOK
	respBody := ""
	kc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = map[string]string{"path": r.URL.Path, "grant_type": r.Form.Get("grant_type"), "username": r.Form.Get("username"),
			"client_id": r.Form.Get("client_id"), "client_secret": r.Form.Get("client_secret")}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	defer kc.Close()
	ctx := context.Background()
	creds := func(secret string) KeycloakCredentials {
		return func(context.Context) (string, string, string) {
			return kc.URL + "/realms/nexus/", "nexus-backend", secret
		}
	}

	r := NewReauthenticator(nil, creds("s3cr3t"))
	if m, err := r.Reauthenticate(ctx, uuid.New(), "joao.silva", true, "senha-do-ad"); err != nil || m != "password_reauth:keycloak" {
		t.Fatalf("senha aceita pelo Keycloak: %q %v", m, err)
	}
	if gotForm["path"] != "/realms/nexus/protocol/openid-connect/token" || gotForm["grant_type"] != "password" ||
		gotForm["username"] != "joao.silva" || gotForm["client_id"] != "nexus-backend" || gotForm["client_secret"] != "s3cr3t" {
		t.Fatalf("requisição ao Keycloak: %+v", gotForm)
	}
	// Client público (sem segredo) não envia client_secret.
	if _, err := NewReauthenticator(nil, creds("")).Reauthenticate(ctx, uuid.New(), "joao", true, "x"); err != nil || gotForm["client_secret"] != "" {
		t.Fatalf("client público: %v %+v", err, gotForm)
	}
	for _, code := range []int{http.StatusUnauthorized, http.StatusBadRequest} {
		status = code
		for _, b := range []string{"", `{"error":"invalid_grant","error_description":"Invalid user credentials"}`, "não-json"} {
			respBody = b
			if _, err := r.Reauthenticate(ctx, uuid.New(), "joao", true, "errada"); !errors.Is(err, domain.ErrReauth) {
				t.Errorf("Keycloak %d %q = senha recusada, veio %v", code, b, err)
			}
		}
	}
	// Erro de client (sem Direct Access Grants, segredo errado) é
	// configuração, não senha errada: 503 em vez de "senha incorreta".
	for code, errCode := range map[int]string{http.StatusBadRequest: "unauthorized_client", http.StatusUnauthorized: "invalid_client"} {
		status, respBody = code, `{"error":"`+errCode+`"}`
		if _, err := r.Reauthenticate(ctx, uuid.New(), "joao", true, "certa"); !unavailable(err) {
			t.Errorf("Keycloak %d %s = configuração (503), veio %v", code, errCode, err)
		}
	}
	respBody = ""
	status = http.StatusInternalServerError
	if _, err := r.Reauthenticate(ctx, uuid.New(), "joao", true, "x"); !unavailable(err) {
		t.Errorf("Keycloak com erro interno vira 503: %v", err)
	}
	// Keycloak fora do ar, não configurado ou com endereço inválido.
	down := NewReauthenticator(nil, func(context.Context) (string, string, string) { return "http://127.0.0.1:1", "c", "" })
	if _, err := down.Reauthenticate(ctx, uuid.New(), "joao", true, "x"); !unavailable(err) {
		t.Errorf("Keycloak inalcançável vira 503: %v", err)
	}
	none := NewReauthenticator(nil, func(context.Context) (string, string, string) { return "", "", "" })
	if _, err := none.Reauthenticate(ctx, uuid.New(), "joao", true, "x"); !unavailable(err) {
		t.Errorf("Keycloak não configurado vira 503: %v", err)
	}
	bad := NewReauthenticator(nil, func(context.Context) (string, string, string) { return "http://\x00invalido", "c", "" })
	if _, err := bad.Reauthenticate(ctx, uuid.New(), "joao", true, "x"); err == nil {
		t.Error("endereço do Keycloak inválido deveria falhar")
	}
}

func unavailable(err error) bool {
	appErr, ok := apperrors.As(err)
	return ok && appErr.Status == http.StatusServiceUnavailable
}
