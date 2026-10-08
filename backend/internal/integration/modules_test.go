// Package integration exercita, contra um PostgreSQL real com todas as
// migrations aplicadas, o fluxo principal de cada plugin do Nexus na
// camada de aplicação (SQL de verdade; storage/reautenticação simulados).
//
// Pulado sem TEST_DATABASE_URL, como os demais testes de integração.
package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	blogApp "github.com/yurythx/projeto-nexus/internal/modules/blog/application"
	blogDomain "github.com/yurythx/projeto-nexus/internal/modules/blog/domain"
	blogInfra "github.com/yurythx/projeto-nexus/internal/modules/blog/infrastructure"
	calApp "github.com/yurythx/projeto-nexus/internal/modules/calendar/application"
	calDomain "github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	calInfra "github.com/yurythx/projeto-nexus/internal/modules/calendar/infrastructure"
	catApp "github.com/yurythx/projeto-nexus/internal/modules/catalog/application"
	catDomain "github.com/yurythx/projeto-nexus/internal/modules/catalog/domain"
	catInfra "github.com/yurythx/projeto-nexus/internal/modules/catalog/infrastructure"
	contactApp "github.com/yurythx/projeto-nexus/internal/modules/contact/application"
	contactDomain "github.com/yurythx/projeto-nexus/internal/modules/contact/domain"
	contactInfra "github.com/yurythx/projeto-nexus/internal/modules/contact/infrastructure"
	dirApp "github.com/yurythx/projeto-nexus/internal/modules/directory/application"
	dirDomain "github.com/yurythx/projeto-nexus/internal/modules/directory/domain"
	dirInfra "github.com/yurythx/projeto-nexus/internal/modules/directory/infrastructure"
	egressApp "github.com/yurythx/projeto-nexus/internal/modules/egress/application"
	egressInfra "github.com/yurythx/projeto-nexus/internal/modules/egress/infrastructure"
	filesApp "github.com/yurythx/projeto-nexus/internal/modules/files/application"
	filesDomain "github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	filesInfra "github.com/yurythx/projeto-nexus/internal/modules/files/infrastructure"
	iamApp "github.com/yurythx/projeto-nexus/internal/modules/iam/application"
	iamDomain "github.com/yurythx/projeto-nexus/internal/modules/iam/domain"
	iamInfra "github.com/yurythx/projeto-nexus/internal/modules/iam/infrastructure"
	mercApp "github.com/yurythx/projeto-nexus/internal/modules/mercurio/application"
	mercInfra "github.com/yurythx/projeto-nexus/internal/modules/mercurio/infrastructure"
	signumApp "github.com/yurythx/projeto-nexus/internal/modules/signum/application"
	signumDomain "github.com/yurythx/projeto-nexus/internal/modules/signum/domain"
	signumInfra "github.com/yurythx/projeto-nexus/internal/modules/signum/infrastructure"
	tramiteApp "github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	tramiteDomain "github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	tramiteInfra "github.com/yurythx/projeto-nexus/internal/modules/tramite/infrastructure"
	wikiApp "github.com/yurythx/projeto-nexus/internal/modules/wiki/application"
	wikiInfra "github.com/yurythx/projeto-nexus/internal/modules/wiki/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/iam"
	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
	"github.com/yurythx/projeto-nexus/internal/platform/netguard"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

var errReauth = signumDomain.ErrReauth

const bucket = "nexus-test"

type env struct {
	pool     *pgxpool.Pool
	ctx      context.Context
	identity auth.Identity
	other    uuid.UUID
	store    *memStorage
	logger   *slog.Logger
	ob       *outbox.Writer
	unidade  uuid.UUID
	unidade2 uuid.UUID
	depto    uuid.UUID
}

// gestaoGlobal: quem gerencia sem escopo (ADR 013) — nexus-admin.
var gestaoGlobal = auth.Identity{Roles: []string{auth.RoleAdmin}}

func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de integração")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	e := &env{pool: pool, ctx: context.Background(), store: newMemStorage(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ob: outbox.NewWriter("nexus.test")}

	iamSvc := iamApp.NewService(pool, iamInfra.NewRepository(), nil)
	suffix := uuid.NewString()[:8]
	// A montagem da estrutura é feita por um administrador (o IAM recusa
	// conceder permissões que o ator não possui).
	e.ctx = auth.WithIdentity(e.ctx, auth.Identity{Username: "bootstrap", Roles: []string{auth.RoleAdmin}})

	ent, err := iamSvc.SaveEntidade(e.ctx, iamDomain.Entidade{Nome: "Entidade " + suffix, Sigla: "ENT", Ativo: true})
	must(t, err)
	un, err := iamSvc.SaveUnidade(e.ctx, iamDomain.Unidade{EntidadeID: ent.ID, Nome: "Unidade A " + suffix, Sigla: "UA", Ativo: true})
	must(t, err)
	un2, err := iamSvc.SaveUnidade(e.ctx, iamDomain.Unidade{EntidadeID: ent.ID, ParentID: &un.ID, Nome: "Unidade B " + suffix, Sigla: "UB", Ativo: true})
	must(t, err)
	dep, err := iamSvc.SaveDepartamento(e.ctx, iamDomain.Departamento{UnidadeID: un.ID, Nome: "Depto " + suffix, Sigla: "DP", Ativo: true})
	must(t, err)
	e.unidade, e.unidade2, e.depto = un.ID, un2.ID, dep.ID

	// Ciclo na hierarquia de unidades é recusado.
	if _, err := iamSvc.SaveUnidade(e.ctx, iamDomain.Unidade{ID: un.ID, EntidadeID: ent.ID, ParentID: &un2.ID, Nome: un.Nome, Ativo: true}); err == nil {
		t.Fatal("hierarquia circular de unidades deveria ser recusada")
	}

	user, err := iamSvc.CreateLocalUser(e.ctx, iamApp.CreateLocalUserInput{
		Username: "tester." + suffix, Email: "tester." + suffix + "@nexus.test", DisplayName: "Tester", Password: "Senha-Forte-123",
	})
	must(t, err)
	other, err := iamSvc.CreateLocalUser(e.ctx, iamApp.CreateLocalUserInput{
		Username: "colega." + suffix, Email: "colega." + suffix + "@nexus.test", DisplayName: "Colega", Password: "Senha-Forte-123",
	})
	must(t, err)
	e.other = other.ID

	perfis, err := iamSvc.ListPerfis(e.ctx)
	must(t, err)
	var protocolo uuid.UUID
	for _, p := range perfis {
		if p.Slug == "protocolo" {
			protocolo = p.ID
		}
	}
	_, err = iamSvc.CreateLotacao(e.ctx, iamDomain.Lotacao{UserID: user.ID, PerfilID: protocolo, Principal: true,
		Scope: iamDomain.Scope{DepartamentoID: &dep.ID}})
	must(t, err)
	_, err = iamSvc.CreateMapping(e.ctx, iamDomain.ADMapping{ADGroup: "GG_Gestores_" + suffix, PerfilID: protocolo,
		Scope: iamDomain.Scope{UnidadeID: &un2.ID}})
	must(t, err)

	// Resolução efetiva pelo IAM: lotação manual (departamento -> unidade
	// completada) + mapeamento do grupo do AD.
	resolver := iam.NewResolver(pool, nil, e.logger)
	identity, err := resolver.Enrich(e.ctx, auth.Identity{
		Subject: user.ID.String(), Username: user.Username, Source: auth.SourceLocal,
		Roles: []string{"nexus-user"}, Groups: []string{"/Nexus/GG_Gestores_" + suffix},
	})
	must(t, err)
	if !auth.HasPermission(identity, auth.PermTramiteCreate) || auth.HasPermission(identity, auth.PermIAMManage) {
		t.Fatalf("permissões efetivas inesperadas: %v", identity.Permissions)
	}
	if !tramiteDomain.InUnidade(identity, un.ID) || !tramiteDomain.InUnidade(identity, un2.ID) {
		t.Fatalf("escopos deveriam cobrir as duas unidades (lotação + AD): %+v", identity.Scopes)
	}
	// Para os demais fluxos, o testador também é administrador (papel
	// nexus-admin = "*" global; "*" só na lista achatada não concede nada
	// no modelo de concessões — ADR 013).
	identity.Roles = append(identity.Roles, auth.RoleAdmin)
	identity.Permissions = append(identity.Permissions, "*")
	e.identity = identity
	e.ctx = auth.WithIdentity(e.ctx, identity)
	return e
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPluginsEndToEnd(t *testing.T) {
	e := setup(t)
	ctx := e.ctx
	p := pagination.New(1, 20, 100)

	t.Run("blog", func(t *testing.T) {
		svc := blogApp.NewService(e.pool, blogInfra.NewRepository(), e.ob, e.store, bucket, time.Minute, e.logger)
		post, err := svc.Create(ctx, gestaoGlobal, blogApp.Input{Title: "Comunicado de Manutenção Programada", Body: "O sistema de protocolo ficará indisponível", Kind: "comunicado"})
		must(t, err)
		post, err = svc.Update(ctx, gestaoGlobal, post.ID, blogApp.Input{Title: post.Title, Body: post.Body + " no sábado.", Kind: "comunicado", Pinned: true})
		must(t, err)
		if _, err := svc.Get(ctx, auth.Identity{}, post.Slug); err == nil {
			t.Fatal("rascunho não pode ser visto sem blog:manage")
		}
		post, err = svc.Transition(ctx, gestaoGlobal, post.ID, "published")
		must(t, err)
		if post.PublishedAt == nil {
			t.Fatal("publicar deve carimbar published_at")
		}
		list, total, err := svc.List(ctx, e.identity, blogDomain.Filter{}, p)
		must(t, err)
		if total < 1 || len(list) < 1 {
			t.Fatal("listagem de publicados vazia")
		}
		res, _, err := svc.Search(ctx, e.identity, "manutenção", 10)
		must(t, err)
		if len(res) == 0 {
			t.Fatal("busca full-text (unaccent + português) deveria achar a publicação")
		}
		assertOutbox(t, e.pool, "blog.post.published", post.ID.String())
		must(t, svc.Delete(ctx, gestaoGlobal, post.ID))
	})

	t.Run("catalog", func(t *testing.T) {
		svc := catApp.NewService(e.pool, catInfra.NewRepository(), e.ob)
		s, err := svc.Save(ctx, gestaoGlobal, uuid.Nil, catDomain.Service{Title: "Emissão de Certidão " + uuid.NewString()[:6], Category: "Documentos",
			Summary: "Solicite certidões", Requirements: []string{"Documento de identidade"},
			Channels: []catDomain.Channel{{Type: "online", Label: "Portal", Value: "https://portal.test"}}, ResponsibleUnidadeID: &e.unidade})
		must(t, err)
		if _, err := svc.GetPublic(ctx, s.Slug); err == nil {
			t.Fatal("rascunho não é público")
		}
		_, err = svc.SetStatus(ctx, gestaoGlobal, s.ID, "published")
		must(t, err)
		got, err := svc.GetPublic(ctx, s.Slug)
		must(t, err)
		if len(got.Channels) != 1 || got.ResponsibleUnidade == "" {
			t.Fatalf("canais/unidade não persistidos: %+v", got)
		}
		cats, err := svc.Categories(ctx)
		must(t, err)
		if len(cats) == 0 {
			t.Fatal("categorias vazias")
		}
		res, _, err := svc.Search(ctx, "certidao", 5)
		must(t, err)
		if len(res) == 0 {
			t.Fatal("busca sem acento deveria achar 'Certidão'")
		}
		assertOutbox(t, e.pool, "catalog.service.published", s.ID.String())
	})

	t.Run("contact", func(t *testing.T) {
		svc := contactApp.NewService(e.pool, contactInfra.NewRepository(), e.ob)
		m, err := svc.Submit(ctx, contactApp.SubmitInput{Name: "Maria", Email: "Maria@Exemplo.com", Subject: "Dúvida",
			Message: "Como solicito uma certidão?", IP: "203.0.113.10", UserAgent: "test"})
		must(t, err)
		if !strings.HasPrefix(m.Protocol, "CT-") {
			t.Fatalf("protocolo inesperado %q", m.Protocol)
		}
		_, total, err := svc.List(ctx, gestaoGlobal, contactDomain.Filter{Status: "new"}, p)
		must(t, err)
		if total < 1 {
			t.Fatal("mensagem não listada")
		}
		_, err = svc.Triage(ctx, gestaoGlobal, m.ID, contactDomain.Triage{Status: "answered", Notes: "Respondido por e-mail"})
		must(t, err)
		var payload []byte
		must(t, e.pool.QueryRow(ctx, `SELECT payload FROM outbox_events WHERE aggregate_id = $1`, m.ID.String()).Scan(&payload))
		if strings.Contains(string(payload), "Maria") || strings.Contains(string(payload), "@") {
			t.Fatal("evento contact.message.submitted não pode carregar PII")
		}
	})

	t.Run("directory", func(t *testing.T) {
		svc := dirApp.NewService(e.pool, dirInfra.NewRepository())
		_, err := svc.SaveProfile(ctx, e.identity, e.identity.UserID, dirDomain.ProfileInput{JobTitle: "Analista", Extension: "1234", Visible: true}, true)
		must(t, err)
		people, _, err := svc.List(ctx, e.identity, dirDomain.Filter{Query: "tester"}, p)
		must(t, err)
		if len(people) == 0 || people[0].JobTitle != "Analista" || people[0].Departamento == "" {
			t.Fatalf("pessoa/lotação não resolvida: %+v", people)
		}
		sectors, err := svc.Sectors(ctx, "")
		must(t, err)
		if len(sectors) < 3 {
			t.Fatalf("setores esperados (2 unidades + 1 depto), veio %d", len(sectors))
		}
	})

	t.Run("calendar", func(t *testing.T) {
		svc := calApp.NewService(e.pool, calInfra.NewRepository(), e.ob)
		room, err := svc.SaveRoom(ctx, gestaoGlobal, uuid.Nil, calDomain.Room{Name: "Sala " + uuid.NewString()[:6], Capacity: 10, Active: true})
		must(t, err)
		start := time.Now().Add(48 * time.Hour).Truncate(time.Hour)
		ev, err := svc.CreateEvent(ctx, e.identity, calApp.EventInput{Title: "Reunião", Visibility: "internal", RoomID: &room.ID, StartsAt: start, EndsAt: start.Add(time.Hour)})
		must(t, err)
		_, err = svc.CreateEvent(ctx, e.identity, calApp.EventInput{Title: "Conflito", Visibility: "internal", RoomID: &room.ID,
			StartsAt: start.Add(30 * time.Minute), EndsAt: start.Add(90 * time.Minute)})
		if err == nil || !strings.Contains(err.Error(), "reservada") {
			t.Fatalf("reserva sobreposta deveria ser recusada pelo EXCLUDE, veio %v", err)
		}
		busy, err := svc.RoomBusy(ctx, room.ID, start.Add(-time.Hour), start.Add(3*time.Hour))
		must(t, err)
		if len(busy) != 1 {
			t.Fatalf("ocupação esperada 1, veio %d", len(busy))
		}
		_, err = svc.CancelEvent(ctx, e.identity, ev.ID)
		must(t, err)
		_, err = svc.CreateEvent(ctx, e.identity, calApp.EventInput{Title: "Agora cabe", Visibility: "public", RoomID: &room.ID, StartsAt: start, EndsAt: start.Add(time.Hour)})
		must(t, err)
		all, err := svc.PublicEvents(ctx, start.Add(-time.Hour), start.Add(2*time.Hour))
		must(t, err)
		var pub []calDomain.Event
		for _, ev := range all {
			if ev.RoomID != nil && *ev.RoomID == room.ID {
				pub = append(pub, ev)
			}
		}
		if len(pub) != 1 || pub[0].OrganizerName != "" {
			t.Fatalf("agenda pública deveria ter 1 evento sem dados do organizador: %+v", pub)
		}
	})

	t.Run("files", func(t *testing.T) {
		svc := filesApp.NewService(e.pool, filesInfra.NewRepository(), e.ob, e.store, bucket, 10<<20, time.Minute, e.logger)
		root, err := svc.CreateFolder(ctx, e.identity, nil, "Raiz "+uuid.NewString()[:6], nil)
		must(t, err)
		sub, err := svc.CreateFolder(ctx, e.identity, &root.ID, "Contratos", nil)
		must(t, err)
		_, err = svc.SetACL(ctx, e.identity, root.ID, []filesDomain.ACLEntry{{SubjectType: "departamento", Subject: e.depto.String()}})
		must(t, err)
		up, err := svc.StartUpload(ctx, e.identity, sub.ID, "contrato.pdf", "application/pdf", 5)
		must(t, err)
		must(t, e.store.Put(ctx, bucket, up.Upload.ObjectKey, strings.NewReader("%PDF-"), 5, "application/pdf"))
		f, err := svc.ConfirmUpload(ctx, e.identity, up.File.ID)
		must(t, err)
		if f.Status != "ready" {
			t.Fatal("arquivo deveria estar pronto")
		}
		// Outro usuário do mesmo departamento lê pela ACL herdada da raiz.
		colega := auth.Identity{UserID: e.other, Scopes: []auth.Scope{{DepartamentoID: &e.depto}}}
		listing, err := svc.Browse(ctx, colega, &sub.ID)
		must(t, err)
		if !listing.Access.Read || listing.Access.Write || len(listing.Files) != 1 || len(listing.Breadcrumbs) != 2 {
			t.Fatalf("acesso herdado inesperado: %+v", listing.Access)
		}
		if _, err := svc.Browse(ctx, auth.Identity{UserID: uuid.New()}, &sub.ID); err == nil {
			t.Fatal("estranho não pode ler a pasta")
		}
		res, err := svc.Search(ctx, colega, "contrato", 5)
		must(t, err)
		if len(res) != 1 {
			t.Fatalf("busca deveria respeitar a ACL e achar 1 arquivo, veio %d", len(res))
		}
		must(t, svc.DeleteFolder(ctx, e.identity, root.ID, true))
		if _, err := e.store.Stat(ctx, bucket, up.Upload.ObjectKey); err == nil {
			t.Fatal("objeto deveria ter saído do storage junto com a pasta")
		}
	})

	t.Run("wiki", func(t *testing.T) {
		svc := wikiApp.NewService(e.pool, wikiInfra.NewRepository())
		parent, err := svc.Create(ctx, e.identity, wikiApp.Input{Title: "Manual " + uuid.NewString()[:6], Body: "# Início"})
		must(t, err)
		child, err := svc.Create(ctx, e.identity, wikiApp.Input{ParentID: &parent.ID, Title: "Procedimento de Férias", Body: "Solicitar com 30 dias"})
		must(t, err)
		updated, err := svc.Update(ctx, e.identity, child.ID, 1, wikiApp.Input{ParentID: &parent.ID, Title: child.Title, Body: "Solicitar com 45 dias"})
		must(t, err)
		if updated.Version != 2 {
			t.Fatalf("versão esperada 2, veio %d", updated.Version)
		}
		if _, err := svc.Update(ctx, e.identity, child.ID, 1, wikiApp.Input{ParentID: &parent.ID, Title: child.Title, Body: "edição concorrente"}); err == nil {
			t.Fatal("edição sobre versão antiga deveria ser recusada (concorrência otimista)")
		}
		if _, err := svc.Update(ctx, e.identity, parent.ID, parent.Version, wikiApp.Input{ParentID: &child.ID, Title: parent.Title}); err == nil {
			t.Fatal("mover a página para dentro da própria filha deveria ser recusado")
		}
		restored, err := svc.Restore(ctx, e.identity, child.ID, 1)
		must(t, err)
		if restored.Body != "Solicitar com 30 dias" || restored.Version != 3 {
			t.Fatalf("restauração inesperada: v%d %q", restored.Version, restored.Body)
		}
		revs, err := svc.Revisions(ctx, e.identity, child.ID)
		must(t, err)
		if len(revs) != 3 {
			t.Fatalf("3 revisões esperadas, veio %d", len(revs))
		}
		view, err := svc.Get(ctx, e.identity, child.Slug)
		must(t, err)
		if len(view.Breadcrumbs) != 1 {
			t.Fatal("trilha até a raiz esperada")
		}
		res, _, err := svc.Search(ctx, e.identity, "ferias", 5)
		must(t, err)
		if len(res) == 0 {
			t.Fatal("busca deveria achar a página")
		}
		if err := svc.Delete(ctx, gestaoGlobal, parent.ID); err == nil {
			t.Fatal("página com filhas não pode ser excluída")
		}
		must(t, svc.Delete(ctx, gestaoGlobal, child.ID))
		must(t, svc.Delete(ctx, gestaoGlobal, parent.ID))
	})

	var signumSvc *signumApp.Service
	t.Run("signum", func(t *testing.T) {
		signumSvc = signumApp.NewService(e.pool, signumInfra.NewRepository(), okReauth{}, nil, e.ob, "segredo-teste", time.Minute, e.logger)
		doc := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
		env, err := signumSvc.Open(ctx, signumApp.OpenRequest{Title: "Termo", DocumentSHA256: doc, SignerIDs: []uuid.UUID{e.identity.UserID}, SourceModule: "signum"})
		must(t, err)
		ch, err := signumSvc.Challenge(ctx, e.identity, env.ID)
		must(t, err)
		if _, err := signumSvc.Sign(ctx, e.identity, env.ID, signumApp.SignInput{ChallengeID: ch.ChallengeID, Nonce: ch.Nonce, Password: "errada", ConfirmSHA256: doc}); err == nil {
			t.Fatal("senha errada não pode assinar")
		}
		if _, err := signumSvc.Sign(ctx, e.identity, env.ID, signumApp.SignInput{ChallengeID: ch.ChallengeID, Nonce: ch.Nonce, Password: "correta", ConfirmSHA256: strings.Repeat("0", 64)}); err == nil {
			t.Fatal("hash de documento divergente não pode assinar")
		}
		signed, err := signumSvc.Sign(ctx, e.identity, env.ID, signumApp.SignInput{ChallengeID: ch.ChallengeID, Nonce: ch.Nonce, Password: "correta", ConfirmSHA256: doc})
		must(t, err)
		if signed.Status != signumDomain.StatusCompleted {
			t.Fatalf("envelope com signatário único deveria concluir, veio %s", signed.Status)
		}
		if _, err := signumSvc.Sign(ctx, e.identity, env.ID, signumApp.SignInput{ChallengeID: ch.ChallengeID, Nonce: ch.Nonce, Password: "correta", ConfirmSHA256: doc}); err == nil {
			t.Fatal("desafio é de uso único e o envelope já fechou")
		}
		v, err := signumSvc.Verify(ctx, env.ID, doc)
		must(t, err)
		if len(v.Signatures) != 1 || !v.Signatures[0].Valid || v.DocumentMatch == nil || !*v.DocumentMatch {
			t.Fatalf("verificação pública inesperada: %+v", v)
		}
		assertOutbox(t, e.pool, "signum.envelope.completed", env.ID.String())
	})

	t.Run("tramite", func(t *testing.T) {
		port := &signaturePort{svc: signumSvc}
		svc := tramiteApp.NewService(e.pool, tramiteInfra.NewRepository(), port, e.ob, e.store, bucket, 10<<20, time.Minute, e.logger)
		tipos, err := svc.Tipos(ctx)
		must(t, err)
		proc, err := svc.Abrir(ctx, e.identity, tramiteApp.AbrirInput{TipoID: tipos[0].ID, Assunto: "Aquisição de notebooks",
			Sigilo: tramiteDomain.SigiloRestrito, UnidadeOrigemID: e.unidade})
		must(t, err)
		if !strings.HasSuffix(proc.Numero, "/"+time.Now().Format("2006")) {
			t.Fatalf("numeração inesperada %q", proc.Numero)
		}
		// Estranho sem lotação não enxerga processo restrito.
		if _, err := svc.Get(ctx, auth.Identity{UserID: e.other}, proc.ID); err == nil {
			t.Fatal("processo restrito não pode ser lido por quem não está nas unidades")
		}
		doc, err := svc.AdicionarDocumento(ctx, e.identity, proc.ID, tramiteApp.NovoDocumentoInput{Titulo: "Termo de referência", Conteudo: "Especificação"})
		must(t, err)
		doc, err = svc.SolicitarAssinatura(ctx, e.identity, doc.ID, []uuid.UUID{e.identity.UserID}, false)
		must(t, err)
		if doc.Status != "aguardando_assinatura" || doc.EnvelopeID == nil || doc.SHA256 == nil {
			t.Fatalf("documento deveria aguardar assinatura com hash congelado: %+v", doc)
		}
		// Simula o evento de conclusão vindo do Signum pelo barramento.
		payload, _ := json.Marshal(map[string]any{"envelope_id": doc.EnvelopeID.String(), "status": "completed", "source_module": "tramite"})
		ev := events.Event{ID: uuid.New(), Type: "signum.envelope.completed", Payload: payload}
		must(t, svc.HandleSignatureEvent(ctx, ev))
		must(t, svc.HandleSignatureEvent(ctx, ev)) // idempotente
		proc2, err := svc.Tramitar(ctx, e.identity, proc.ID, e.unidade2, "Encaminho para análise")
		must(t, err)
		if proc2.UnidadeAtualID != e.unidade2 || proc2.Status != tramiteDomain.StatusEmTramitacao {
			t.Fatalf("tramitação inesperada: %+v", proc2)
		}
		view, err := svc.Get(ctx, e.identity, proc.ID)
		must(t, err)
		if view.Documentos[0].Status != "assinado" || len(view.Movimentos) < 4 {
			t.Fatalf("documento deveria estar assinado e o histórico completo: %+v / %d movimentos", view.Documentos[0].Status, len(view.Movimentos))
		}
		// Histórico é append-only.
		if _, err := e.pool.Exec(ctx, `UPDATE tramite_movimentos SET despacho = 'x' WHERE processo_id = $1`, proc.ID); err == nil {
			t.Fatal("tramite_movimentos deveria ser imutável")
		}
		_, err = svc.Concluir(ctx, e.identity, proc.ID, "Concluído")
		must(t, err)
		_, err = svc.Arquivar(ctx, e.identity, proc.ID, "Arquivado")
		must(t, err)
		list, _, err := svc.List(ctx, e.identity, tramiteDomain.Filter{Query: "notebooks"}, p)
		must(t, err)
		if len(list) == 0 {
			t.Fatal("busca de processos deveria achar")
		}
	})

	t.Run("mercurio", func(t *testing.T) {
		svc := mercApp.NewService(e.pool, mercInfra.NewRepository(), ws.NewHub(e.logger, nil), e.ob, e.logger)
		rooms, err := svc.Rooms(ctx, e.identity)
		must(t, err)
		var global uuid.UUID
		for _, r := range rooms {
			if r.Kind == "global" {
				global = r.ID
			}
		}
		if global == uuid.Nil {
			t.Fatal("sala global semeada não encontrada")
		}
		m, err := svc.Send(ctx, e.identity, global, "Olá, equipe!")
		must(t, err)
		_, err = svc.Edit(ctx, e.identity, m.ID, "Olá, equipe! (editado)")
		must(t, err)
		dm, err := svc.Direct(ctx, e.identity, e.other)
		must(t, err)
		dm2, err := svc.Direct(ctx, e.identity, e.other)
		must(t, err)
		if dm.ID != dm2.ID {
			t.Fatal("conversa direta deve ser única por par")
		}
		_, err = svc.Send(ctx, e.identity, dm.ID, "mensagem privada")
		must(t, err)
		if _, err := svc.Messages(ctx, auth.Identity{UserID: uuid.New()}, dm.ID, nil, 10); err == nil {
			t.Fatal("terceiro não lê conversa direta")
		}
		msgs, err := svc.Messages(ctx, e.identity, global, nil, 10)
		must(t, err)
		if len(msgs) == 0 || msgs[len(msgs)-1].EditedAt == nil {
			t.Fatal("mensagem editada não retornada")
		}
		must(t, svc.MarkRead(ctx, e.identity, global))
		must(t, svc.Delete(ctx, e.identity, m.ID))
	})

	t.Run("egress", func(t *testing.T) {
		var received struct {
			sig, event string
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received.sig, received.event = r.Header.Get("X-Nexus-Signature"), r.Header.Get("X-Nexus-Event")
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		cipher, err := secretcrypto.NewFromBase64Key("MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=")
		must(t, err)
		policy := netguard.Policy{AllowPrivate: true, AllowHTTP: true}
		svc := egressApp.NewService(e.pool, egressInfra.NewRepository(), egressInfra.NewDeliverer(2*time.Second, policy), cipher,
			egressApp.Config{MaxAttempts: 3, BatchSize: 10, PollInterval: time.Second}, e.logger)
		secret := "s3cr3t"
		target, err := svc.SaveTarget(ctx, uuid.Nil, egressApp.TargetInput{Name: "n8n", Kind: "n8n", URL: srv.URL + "/hook",
			Secret: &secret, EventPatterns: []string{"blog.#"}, Active: true})
		must(t, err)
		if !target.HasSecret {
			t.Fatal("segredo deveria ter sido gravado (cifrado)")
		}
		ev := events.Event{ID: uuid.New(), Type: "blog.post.published", Payload: json.RawMessage(`{"id":"x"}`), OccurredAt: time.Now()}
		must(t, svc.Dispatch(ctx, ev))
		must(t, svc.Dispatch(ctx, ev)) // reentrega do broker não duplica
		must(t, svc.Dispatch(ctx, events.Event{ID: uuid.New(), Type: "contact.message.submitted", Payload: json.RawMessage(`{}`)}))
		dels, total, err := svc.Deliveries(ctx, &target.ID, "", p)
		must(t, err)
		if total != 1 || len(dels) != 1 {
			t.Fatalf("1 entrega esperada (padrão blog.# + dedup), veio %d", total)
		}
		// Isolamento: entregas pendentes deixadas por execuções anteriores no
		// mesmo banco (destinos de servidores de teste já encerrados) não
		// podem disputar o lote do worker com a entrega deste teste.
		if _, err := e.pool.Exec(ctx, `UPDATE egress_targets SET active = false WHERE id <> $1`, target.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.pool.Exec(ctx, `UPDATE egress_deliveries SET status = 'dead' WHERE target_id <> $1 AND status IN ('pending','failed')`, target.ID); err != nil {
			t.Fatal(err)
		}
		runCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		_ = svc.RunDeliveries(runCtx)
		cancel()
		dels, _, err = svc.Deliveries(ctx, &target.ID, "delivered", p)
		must(t, err)
		if len(dels) != 1 || received.event != "blog.post.published" || !strings.HasPrefix(received.sig, "t=") {
			t.Fatalf("entrega não concluída/assinada: %+v %+v", dels, received)
		}
		strict := egressApp.NewService(e.pool, egressInfra.NewRepository(), egressInfra.NewDeliverer(time.Second, netguard.Policy{}), cipher, egressApp.Config{}, e.logger)
		if _, err := strict.SaveTarget(ctx, uuid.Nil, egressApp.TargetInput{Name: "x", Kind: "webhook", URL: "https://169.254.169.254/latest", Active: true}); err == nil {
			t.Fatal("destino de metadata de nuvem deveria ser recusado (SSRF)")
		}
	})

	t.Run("kernel_store", func(t *testing.T) {
		st := kernel.NewPostgresStore(e.pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
		key := "test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
		must(t, st.Ensure(ctx, key, true))
		must(t, st.Set(ctx, key, false, "tester", audit.Meta(ctx, audit.ActionModuleToggled, "module", key, nil, nil)))
		state, err := st.LoadAll(ctx)
		must(t, err)
		if v, ok := state[key]; !ok || v {
			t.Fatal("estado do módulo não persistido")
		}
		_, err = e.pool.Exec(ctx, `DELETE FROM system_modules WHERE key = $1`, key)
		must(t, err)
	})

	t.Run("audit_chain_intact", func(t *testing.T) {
		res, err := audit.NewReader(e.pool).Verify(ctx, 1, 0)
		must(t, err)
		if !res.Valid || res.Checked < 20 {
			t.Fatalf("cadeia de auditoria deveria estar íntegra após todos os fluxos: %+v", res)
		}
		var withActor int
		must(t, e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id = $1 AND entity_context ? 'scopes'`, e.identity.UserID).Scan(&withActor))
		if withActor == 0 {
			t.Fatal("auditoria deveria registrar ator e contexto organizacional")
		}
	})
}

func assertOutbox(t *testing.T, pool *pgxpool.Pool, eventType, aggregateID string) {
	t.Helper()
	var n int
	must(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2`,
		eventType, aggregateID).Scan(&n))
	if n == 0 {
		t.Fatalf("evento %s não gravado no Outbox para %s", eventType, aggregateID)
	}
}

// signaturePort replica o adaptador de internal/app (Trâmite -> Signum).
type signaturePort struct{ svc *signumApp.Service }

func (p *signaturePort) Available() bool { return true }

func (p *signaturePort) OpenEnvelope(ctx context.Context, tx pgx.Tx, req tramiteDomain.SignatureRequest) (uuid.UUID, error) {
	env, err := p.svc.OpenTx(ctx, tx, signumApp.OpenRequest{Title: req.Title, DocumentSHA256: req.DocumentSHA256,
		SignerIDs: req.SignerIDs, SourceModule: "tramite", SourceRef: req.SourceRef})
	if err != nil {
		return uuid.Nil, err
	}
	return env.ID, nil
}
