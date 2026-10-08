// Package transparency expõe dados de interesse público em formato aberto
// (JSON, CSV, XML), SEM autenticação — transparência ativa (LAI, Lei
// 12.527/2011, art. 8º; LC 131/2009). Rotas montadas em
// /api/v1/transparencia/*, rate-limited por IP.
//
// F3.5 do roadmap de conformidade — SCAFFOLD: o órgão define quais
// datasets de interesse público de fato publicar. Os datasets aqui são
// exemplos deliberadamente seguros (agregados, nunca dado pessoal):
//   - GET /transparencia/datasets        → catálogo + dicionário de dados
//   - GET /transparencia/plataforma      → métricas agregadas da plataforma
//   - GET /transparencia/modulos         → módulos (plugins) ativos da plataforma
//   - GET /transparencia/auditoria/acoes → catálogo de ações auditadas + volume
//
// Todos aceitam ?format=json|csv|xml (default: json) ou o header Accept.
package transparency

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/lgpd"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ModuleInfo é a visão pública de um módulo.
type ModuleInfo struct {
	Key, Name, Description string
	Enabled                bool
}

// Service publica os datasets de transparência ativa.
type Service struct {
	db      database.DBTX // o pool, em produção
	logger  *slog.Logger
	modules func() []ModuleInfo
}

// NewService cria o serviço; modules lista o estado dos plugins (Kernel).
func NewService(db *pgxpool.Pool, logger *slog.Logger, modules func() []ModuleInfo) *Service {
	return &Service{db: db, logger: logger, modules: modules}
}

// RegisterRoutes monta as rotas públicas de transparência. r deve ser um
// router SEM auth.RequireAuthentication; limiter aplica rate limit por IP.
func RegisterRoutes(r chi.Router, s *Service, limiter httpserver.Limiter) {
	r.Route("/transparencia", func(t chi.Router) {
		t.Use(httpserver.RateLimit(s.logger, limiter, httpserver.ClientIPKey))
		t.Get("/datasets", s.handleCatalog)
		t.Get("/plataforma", s.handlePlatform)
		t.Get("/modulos", s.handleModules)
		t.Get("/auditoria/acoes", s.handleAuditActions)
	})
}

// ---- catálogo / dicionário de dados -------------------------------------

func (s *Service) handleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := []map[string]any{
		{
			"id": "plataforma", "titulo": "Métricas agregadas da plataforma",
			"endpoint": "/api/v1/transparencia/plataforma",
			"campos": map[string]string{
				"usuarios_ativos":      "inteiro — contagem de contas ativas (sem identificação)",
				"modulos_ativos":       "inteiro — total de módulos (plugins) ativos",
				"termos_lgpd_vigentes": "texto — versão vigente dos Termos/Política de Privacidade",
				"gerado_em":            "data-hora UTC (RFC3339)",
			},
			"periodicidade": "tempo real", "formatos": []string{"json", "csv", "xml"},
		},
		{
			"id": "modulos", "titulo": "Módulos da plataforma e disponibilidade",
			"endpoint": "/api/v1/transparencia/modulos",
			"campos": map[string]string{
				"chave": "texto", "nome": "texto", "descricao": "texto", "ativo": "booleano",
			},
			"periodicidade": "tempo real", "formatos": []string{"json", "csv", "xml"},
		},
		{
			"id": "auditoria_acoes", "titulo": "Ações registradas na trilha de auditoria (agregado)",
			"endpoint": "/api/v1/transparencia/auditoria/acoes?dias=30",
			"campos": map[string]string{
				"acao":         "texto — identificador da ação (ex.: login, user.created)",
				"ocorrencias":  "inteiro — quantas vezes no período",
				"periodo_dias": "inteiro — janela consultada (default 30, máx 365)",
			},
			"periodicidade": "tempo real", "formatos": []string{"json", "csv", "xml"},
		},
	}
	s.write(w, r, "catalogo_transparencia", map[string]any{
		"nota": "Transparência ativa (LAI art. 8º). Datasets de exemplo, agregados e sem dado pessoal — " +
			"o órgão define os datasets de interesse público a publicar.",
	}, catalog)
}

// ---- plataforma -------------------------------------------------------

func (s *Service) handlePlatform(w http.ResponseWriter, r *http.Request) {
	var usuariosAtivos, modulosAtivos int
	// Dado público: se a contagem falha, responde erro — nunca publica um
	// "0 usuários ativos" que não é verdade.
	if err := s.db.QueryRow(r.Context(), `SELECT count(*) FROM users WHERE active`).Scan(&usuariosAtivos); err != nil {
		httputil.WriteError(w, r, s.logger, apperrors.Internal(fmt.Errorf("transparency platform: %w", err)))
		return
	}
	for _, m := range s.modules() {
		if m.Enabled {
			modulosAtivos++
		}
	}

	row := map[string]any{
		"usuarios_ativos":      usuariosAtivos,
		"modulos_ativos":       modulosAtivos,
		"termos_lgpd_vigentes": lgpd.CurrentTermVersion,
		"gerado_em":            time.Now().UTC().Format(time.RFC3339),
	}
	s.write(w, r, "plataforma", nil, []map[string]any{row})
}

// ---- módulos ---------------------------------------------------------

func (s *Service) handleModules(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, m := range s.modules() {
		out = append(out, map[string]any{"chave": m.Key, "nome": m.Name, "descricao": m.Description, "ativo": m.Enabled})
	}
	s.write(w, r, "modulos", nil, out)
}

// ---- ações auditadas (agregado) -----------------------------------

func (s *Service) handleAuditActions(w http.ResponseWriter, r *http.Request) {
	dias := 30
	if v := r.URL.Query().Get("dias"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			httputil.WriteError(w, r, s.logger, apperrors.BadRequest("dias deve ser um inteiro entre 1 e 365"))
			return
		}
		dias = n
	}
	since := time.Now().UTC().AddDate(0, 0, -dias)
	rows, err := s.db.Query(r.Context(), `
		SELECT action, count(*) FROM audit_logs
		WHERE created_at >= $1
		GROUP BY action ORDER BY count(*) DESC`, since)
	if err != nil {
		httputil.WriteError(w, r, s.logger, apperrors.Internal(fmt.Errorf("transparency audit actions: %w", err)))
		return
	}
	// Sai completo ou não sai: pular linhas daria números públicos errados.
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (map[string]any, error) {
		var action string
		var n int
		err := row.Scan(&action, &n)
		return map[string]any{"acao": action, "ocorrencias": n, "periodo_dias": dias}, err
	})
	if err != nil {
		httputil.WriteError(w, r, s.logger, apperrors.Internal(fmt.Errorf("transparency audit actions: %w", err)))
		return
	}
	s.write(w, r, "auditoria_acoes", map[string]any{"periodo_dias": dias, "desde": since.Format(time.RFC3339)}, out)
}

// ---- serialização multi-formato --------------------------------------

func (s *Service) write(w http.ResponseWriter, r *http.Request, name string, meta map[string]any, rows []map[string]any) {
	format := chooseFormat(r)
	w.Header().Set("X-Dataset-Name", name)
	w.Header().Set("X-Generated-At", time.Now().UTC().Format(time.RFC3339))
	w.Header().Set("Cache-Control", "public, max-age=60")

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		cw := csv.NewWriter(w)
		if len(rows) > 0 {
			cols := sortedKeys(rows[0])
			_ = cw.Write(cols)
			for _, row := range rows {
				rec := make([]string, len(cols))
				for i, c := range cols {
					rec[i] = fmt.Sprintf("%v", row[c])
				}
				_ = cw.Write(rec)
			}
		}
		cw.Flush()
	case "xml":
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(xml.Header))
		_, _ = w.Write([]byte("<dataset name=\"" + xmlEscape(name) + "\">\n"))
		for _, row := range rows {
			_, _ = w.Write([]byte("  <registro>\n"))
			for _, c := range sortedKeys(row) {
				// Valores escapados: "Pessoas & Setores" quebraria o XML.
				_, _ = fmt.Fprintf(w, "    <%s>%s</%s>\n", c, xmlEscape(fmt.Sprintf("%v", row[c])), c)
			}
			_, _ = w.Write([]byte("  </registro>\n"))
		}
		_, _ = w.Write([]byte("</dataset>\n"))
	default: // json
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		body := map[string]any{"dataset": name, "registros": rows}
		if meta != nil {
			body["meta"] = meta
		}
		_ = json.NewEncoder(w).Encode(body)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func xmlEscape(v string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(v)) // strings.Builder não falha
	return b.String()
}

func chooseFormat(r *http.Request) string {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))) {
	case "csv":
		return "csv"
	case "xml":
		return "xml"
	case "json":
		return "json"
	}
	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "text/csv"):
		return "csv"
	case strings.Contains(accept, "application/xml"), strings.Contains(accept, "text/xml"):
		return "xml"
	default:
		return "json"
	}
}
