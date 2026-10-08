package app

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas"
	"github.com/yurythx/projeto-nexus/internal/modules/auditoria"
	"github.com/yurythx/projeto-nexus/internal/modules/blog"
	"github.com/yurythx/projeto-nexus/internal/modules/busca"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar"
	"github.com/yurythx/projeto-nexus/internal/modules/catalog"
	"github.com/yurythx/projeto-nexus/internal/modules/contact"
	"github.com/yurythx/projeto-nexus/internal/modules/directory"
	"github.com/yurythx/projeto-nexus/internal/modules/egress"
	"github.com/yurythx/projeto-nexus/internal/modules/example"
	"github.com/yurythx/projeto-nexus/internal/modules/files"
	"github.com/yurythx/projeto-nexus/internal/modules/iam"
	iamTransport "github.com/yurythx/projeto-nexus/internal/modules/iam/transport"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio"
	"github.com/yurythx/projeto-nexus/internal/modules/signum"
	signumApp "github.com/yurythx/projeto-nexus/internal/modules/signum/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite"
	tramiteDomain "github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/wiki"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
)

// registerPlugins monta o catálogo de módulos (skill §6) e o registra no
// Kernel via RegisterModule. A ordem é a do menu.
func registerPlugins(d *Dependencies) {
	deps := modkit.Deps{
		Config:                d.Config,
		Logger:                d.Logger,
		Pool:                  d.DB,
		Outbox:                d.Outbox,
		Storage:               d.Storage,
		Hub:                   d.Hub,
		Notificacoes:          d.Notificacoes,
		Cipher:                d.Cipher,
		PublicLimiter:         d.RateLimiters.Public,
		ContactLimiter:        d.RateLimiters.Contact,
		AtlasChatLimiter:      d.RateLimiters.AtlasChat,
		InvalidatePermissions: d.IAM.Invalidate,
		ResetLoginLockout: func(ctx context.Context, username string) error {
			return d.RateLimiters.Lockout.Reset(ctx, "user:"+strings.ToLower(username))
		},
	}

	signumModule := signum.New(deps, d.keycloakCredentials, d.RateLimiters.Lockout)

	d.Kernel.MustRegister(
		// Core imutável (nunca desativável).
		iam.New(deps, d.permissionCatalog),
		auditoria.New(deps),
		// Plugins (ativação em runtime).
		mercurio.New(deps),
		egress.New(deps),
		blog.New(deps),
		catalog.New(deps),
		contact.New(deps),
		directory.New(deps),
		calendar.New(deps),
		files.New(deps),
		wiki.New(deps),
		busca.New(deps, d.Kernel.SearchProviders),
		signumModule,
		tramite.New(deps, &signaturePort{svc: signumModule.Service(), enabled: func() bool { return d.Kernel.Enabled(signum.Key) }}),
		atlas.New(deps),
		example.New(deps),
	)
	d.LGPD.SetPersonalDataSource(d.personalData)
}

// personalData lista os plugins que guardam dados pessoais do titular
// (LGPD art. 18) — todos os registrados, ativos ou não.
func (d *Dependencies) personalData() map[string]lgpd.PersonalData {
	out := map[string]lgpd.PersonalData{}
	for _, p := range d.Kernel.Plugins() {
		if pd, ok := p.(lgpd.PersonalData); ok {
			out[p.Manifest().Key] = pd
		}
	}
	return out
}

// permissionCatalog agrupa as permissões declaradas nos manifestos (tela
// de Perfis).
func (d *Dependencies) permissionCatalog() []iamTransport.PermissionGroup {
	out := []iamTransport.PermissionGroup{}
	for _, s := range d.Kernel.Status() {
		if len(s.Permissions) == 0 {
			continue
		}
		g := iamTransport.PermissionGroup{Module: s.Key, Name: s.Name}
		for _, p := range s.Permissions {
			g.Permissions = append(g.Permissions, iamTransport.PermissionItem{Key: p.Key, Description: p.Description})
		}
		out = append(out, g)
	}
	return out
}

// keycloakCredentials fornece ao Signum o client usado na reautenticação
// (configuração dinâmica salva pela tela; fallback no .env).
func (d *Dependencies) keycloakCredentials(ctx context.Context) (string, string, string) {
	if s, err := d.KeycloakCfg.Get(ctx); err == nil && s.Configured {
		return s.IssuerURL, s.ClientID, s.ClientSecret
	}
	k := d.Config.Keycloak
	return k.IssuerURL, k.ClientID, k.ClientSecret
}

// signaturePort liga a porta do Trâmite ao Signum — o único ponto em que
// os dois plugins se encontram. O envelope nasce na MESMA transação que
// muda o status do documento no Trâmite.
type signaturePort struct {
	svc     *signumApp.Service
	enabled func() bool
}

func (p *signaturePort) Available() bool { return p.enabled() }

func (p *signaturePort) OpenEnvelope(ctx context.Context, tx pgx.Tx, req tramiteDomain.SignatureRequest) (uuid.UUID, error) {
	env, err := p.svc.OpenTx(ctx, tx, signumApp.OpenRequest{
		Title: req.Title, Description: req.Description, DocumentSHA256: req.DocumentSHA256,
		SignerIDs: req.SignerIDs, Sequential: req.Sequential, SourceModule: tramite.Key, SourceRef: req.SourceRef,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return env.ID, nil
}
