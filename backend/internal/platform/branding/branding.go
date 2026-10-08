// Package branding guarda a identidade visual white-label da instalação
// (nome, órgão, contatos, logo e design tokens de cor). O frontend aplica
// os tokens como CSS custom properties sobre a paleta DSGov padrão, o que
// permite personalizar a plataforma para diferentes órgãos e empresas sem
// alterar código.
package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Settings é a configuração de branding.
type Settings struct {
	AppName        string            `json:"app_name" validate:"required,max=80"`
	AppDescription string            `json:"app_description" validate:"max=200"`
	OrgName        string            `json:"org_name" validate:"required,max=120"`
	LogoURL        string            `json:"logo_url" validate:"omitempty,max=500"`
	FaviconURL     string            `json:"favicon_url" validate:"omitempty,max=500"`
	SupportEmail   string            `json:"support_email" validate:"omitempty,email,max=200"`
	SupportPhone   string            `json:"support_phone" validate:"max=40"`
	SupportHours   string            `json:"support_hours" validate:"max=120"`
	Tokens         map[string]string `json:"tokens"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Tokens aceitos (nomes de design tokens do frontend) e formato de cor.
var (
	allowedTokens = map[string]struct{}{
		"primary": {}, "primary-foreground": {}, "secondary": {}, "accent": {},
		"header-topbar": {}, "header-bg": {}, "footer-bg": {}, "footer-foreground": {},
		"success": {}, "warning": {}, "danger": {}, "info": {},
	}
	hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// https:// ou caminho LOCAL ("/..."). "//host" e "/\host" são URLs
	// externas para o navegador e ficam de fora.
	safeURL = regexp.MustCompile(`^(https://|/[^/\\])[^\s"'<>\\]*$`)

	// contrastPairs são as combinações texto/fundo que a interface usa: com
	// as duas cores definidas, o contraste precisa atender o WCAG 2.1 AA
	// (4,5:1) exigido pelo e-MAG.
	contrastPairs = [][2]string{{"primary-foreground", "primary"}, {"footer-foreground", "footer-bg"}}
)

// minContrast é o contraste mínimo do WCAG 2.1 AA para texto normal.
const minContrast = 4.5

// Validate confere tokens e URLs (a URL de logo vai parar num <img src>:
// só https ou caminho relativo, nunca javascript:/data:).
func (s Settings) Validate() error {
	if err := httputil.Validate(s); err != nil {
		return err
	}
	for k, v := range s.Tokens {
		if _, ok := allowedTokens[k]; !ok {
			return apperrors.Validation(fmt.Sprintf("token de design desconhecido: %q", k))
		}
		if !hexColor.MatchString(v) {
			return apperrors.Validation(fmt.Sprintf("token %q precisa ser uma cor hexadecimal (#RRGGBB)", k))
		}
	}
	for _, pair := range contrastPairs {
		fg, okFg := s.Tokens[pair[0]]
		bg, okBg := s.Tokens[pair[1]]
		if okFg && okBg {
			if ratio := contrast(fg, bg); ratio < minContrast {
				return apperrors.Validation(fmt.Sprintf(
					"contraste entre %q e %q é %.2f:1 — o e-MAG exige pelo menos 4,5:1", pair[0], pair[1], ratio))
			}
		}
	}
	for field, v := range map[string]string{"logo_url": s.LogoURL, "favicon_url": s.FaviconURL} {
		if v != "" && !safeURL.MatchString(v) {
			return apperrors.Validation(field + " precisa ser https:// ou um caminho relativo")
		}
	}
	return nil
}

// contrast é a razão de contraste WCAG entre duas cores #RGB/#RRGGBB.
func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance é a luminância relativa (WCAG 2.1) de uma cor já validada.
func luminance(hex string) float64 {
	h := strings.TrimPrefix(hex, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	var rgb [3]float64
	for i := range rgb {
		v, _ := strconv.ParseUint(h[i*2:i*2+2], 16, 8) // já passou pelo hexColor
		c := float64(v) / 255
		if c <= 0.03928 {
			rgb[i] = c / 12.92
		} else {
			rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
}

// Store lê/grava branding_settings.
type Store struct {
	pool *pgxpool.Pool
	db   database.DBTX // leituras avulsas (o pool, em produção)
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool, db: pool} }

// Get lê a configuração atual.
func (s *Store) Get(ctx context.Context) (Settings, error) {
	return s.get(ctx, s.db)
}

func (s *Store) get(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (Settings, error) {
	var out Settings
	var tokens []byte
	err := q.QueryRow(ctx, `
		SELECT app_name, app_description, org_name, logo_url, favicon_url,
		       support_email, support_phone, support_hours, tokens, updated_at
		FROM branding_settings WHERE id = 'default'`).
		Scan(&out.AppName, &out.AppDescription, &out.OrgName, &out.LogoURL, &out.FaviconURL,
			&out.SupportEmail, &out.SupportPhone, &out.SupportHours, &tokens, &out.UpdatedAt)
	if err != nil {
		return Settings{}, fmt.Errorf("branding: get: %w", err)
	}
	out.Tokens = map[string]string{}
	_ = json.Unmarshal(tokens, &out.Tokens)
	return out, nil
}

// Save grava a configuração e a auditoria (com diff) na mesma transação.
func (s *Store) Save(ctx context.Context, in Settings, actor string) (Settings, error) {
	var saved Settings
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		saved, err = s.save(ctx, tx, in, actor)
		return err
	})
	return saved, err
}

func (s *Store) save(ctx context.Context, tx database.DBTX, in Settings, actor string) (Settings, error) {
	before, err := s.get(ctx, tx)
	if err != nil {
		return Settings{}, err
	}
	tokens, _ := json.Marshal(in.Tokens) // map[string]string sempre serializa
	if _, err := tx.Exec(ctx, `
		UPDATE branding_settings SET app_name=$1, app_description=$2, org_name=$3, logo_url=$4,
		       favicon_url=$5, support_email=$6, support_phone=$7, support_hours=$8, tokens=$9,
		       updated_at=now(), updated_by=$10
		WHERE id = 'default'`,
		in.AppName, in.AppDescription, in.OrgName, in.LogoURL, in.FaviconURL,
		in.SupportEmail, in.SupportPhone, in.SupportHours, tokens, actor); err != nil {
		return Settings{}, fmt.Errorf("branding: save: %w", err)
	}
	saved, err := s.get(ctx, tx)
	if err != nil {
		return Settings{}, err
	}
	return saved, audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "branding.updated", "branding", "default", before, saved))
}

// Handlers expõe o branding.
type Handlers struct {
	store  *Store
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(store *Store, logger *slog.Logger) *Handlers {
	return &Handlers{store: store, logger: logger}
}

// RegisterPublicRoutes — GET /branding é público: a tela de login e o
// site público já precisam do nome/cores do órgão.
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/branding", h.Get)
}

// RegisterAdminRoutes — PUT /admin/branding (branding:manage).
func (h *Handlers) RegisterAdminRoutes(r chi.Router) {
	r.With(auth.RequirePermission(h.logger, auth.PermBrandingManage)).Put("/admin/branding", h.Put)
}

// Get — GET /branding
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.Get(r.Context())
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteOK(w, s)
}

// Put — PUT /admin/branding
func (h *Handlers) Put(w http.ResponseWriter, r *http.Request) {
	var in Settings
	if err := httputil.DecodeJSON(w, r, &in); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	if in.Tokens == nil {
		in.Tokens = map[string]string{}
	}
	if err := in.Validate(); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	saved, err := h.store.Save(r.Context(), in, identity.Username)
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOK(w, saved)
}
