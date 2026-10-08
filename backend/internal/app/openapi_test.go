package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	yaml "go.yaml.in/yaml/v3"

	"github.com/yurythx/projeto-nexus/internal/platform/kernel"
)

// O contrato OpenAPI (docs/openapi.yaml, servido como backend/docs/openapi.json
// em /docs) precisa descrever EXATAMENTE as rotas que o roteador monta —
// interoperabilidade e-PING. Este teste falha quando um lado tem rota que o
// outro não tem. Para regenerar (preservando as operações já descritas à
// mão e gerando o esqueleto das novas):
//
//	UPDATE_OPENAPI=1 TEST_DATABASE_URL=... go test ./internal/app -run TestOpenAPIMatchesRouter

const (
	openapiJSON = "../../docs/openapi.json"
	openapiYAML = "../../../docs/openapi.yaml"
)

// Caminhos fora da verificação automática (servem a própria documentação,
// ou são montados para todos os métodos — /metrics mantém a descrição
// escrita à mão, se houver).
var openapiIgnored = map[string]bool{"/openapi.json": true, "/docs": true, "/docs/assets/*": true, "/metrics": true}

var chiParam = regexp.MustCompile(`\{([^}:]+)(:[^}]+)?\}`)

type routeInfo struct {
	method, path string // path no formato OpenAPI (/x/{id})
	module       string // chave do plugin dono (vazio = núcleo da plataforma)
	handler      string // função que atende (nome em runtime), para os schemas
}

func collectRoutes(t *testing.T, h *apiHarness) []routeInfo {
	t.Helper()
	owner := map[string]string{}
	for _, p := range h.d.Kernel.Plugins() {
		for _, r := range pluginRoutes(t, p) {
			owner[r.method+" "+r.path] = p.Manifest().Key
		}
	}
	var out []routeInfo
	_ = chi.Walk(h.router.(chi.Router), func(method, route string, handler http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		if route == "" {
			route = "/"
		}
		key := method + " " + route
		if openapiIgnored[route] {
			return nil
		}
		path := chiParam.ReplaceAllString(route, "{$1}")
		out = append(out, routeInfo{method: method, path: path, module: owner[key], handler: handlerName(handler)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].path+out[i].method < out[j].path+out[j].method })
	return out
}

func TestOpenAPIMatchesRouter(t *testing.T) {
	h := newHarness(t)
	routes := collectRoutes(t, h)

	raw, err := os.ReadFile(openapiJSON)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)

	documented := map[string]bool{}
	for p, ops := range paths {
		if openapiIgnored[p] {
			continue
		}
		for m := range ops.(map[string]any) {
			if isMethod(m) {
				documented[strings.ToUpper(m)+" "+p] = true
			}
		}
	}
	mounted := map[string]bool{}
	for _, r := range routes {
		mounted[r.method+" "+r.path] = true
	}

	if os.Getenv("UPDATE_OPENAPI") == "1" {
		regenerate(t, h, spec, routes)
		return
	}
	var missing, stale []string
	for k := range mounted {
		if !documented[k] {
			missing = append(missing, k)
		}
	}
	for k := range documented {
		if !mounted[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing)+len(stale) > 0 {
		t.Fatalf("OpenAPI fora de sincronia com o roteador (rode com UPDATE_OPENAPI=1).\nsem documentação: %v\ndocumentadas mas inexistentes: %v", missing, stale)
	}

	// Todo objeto de resposta precisa de description (OpenAPI 3.0).
	for p, ops := range paths {
		for m, op := range ops.(map[string]any) {
			om, _ := op.(map[string]any)
			if !isMethod(m) || om == nil {
				continue
			}
			resps, _ := om["responses"].(map[string]any)
			for code, r := range resps {
				rm, _ := r.(map[string]any)
				if _, ref := rm["$ref"]; !ref && rm["description"] == nil {
					t.Errorf("%s %s: resposta %s sem description", strings.ToUpper(m), p, code)
				}
			}
		}
	}

	// Os schemas gerados precisam refletir os tipos Go de hoje.
	var expected map[string]any
	_ = json.Unmarshal(raw, &expected)
	applySchemas(t, expected, routes)
	want, _ := json.Marshal(expected)
	got, _ := json.Marshal(spec)
	if string(want) != string(got) {
		t.Fatal("schemas do OpenAPI desatualizados em relação aos tipos Go (rode com UPDATE_OPENAPI=1)")
	}
}

func isMethod(m string) bool {
	switch strings.ToLower(m) {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	}
	return false
}

// regenerate reescreve o contrato: mantém as operações existentes, gera as
// novas e remove as que não existem mais.
func regenerate(t *testing.T, h *apiHarness, spec map[string]any, routes []routeInfo) {
	t.Helper()
	_, plainTok := h.user("nexus-user")
	oldPaths, _ := spec["paths"].(map[string]any)
	newPaths := map[string]any{}
	manifests := map[string]kernel.Manifest{}
	for _, p := range h.d.Kernel.Plugins() {
		manifests[p.Manifest().Key] = p.Manifest()
	}
	usedTags := map[string]bool{}

	for _, r := range routes {
		ops, _ := newPaths[r.path].(map[string]any)
		if ops == nil {
			ops = map[string]any{}
			newPaths[r.path] = ops
		}
		lm := strings.ToLower(r.method)
		if old, ok := oldPaths[r.path].(map[string]any); ok {
			if op, ok := old[lm]; ok {
				ops[lm] = op
				for _, tag := range tagsOf(op) {
					usedTags[tag] = true
				}
				continue
			}
		}
		op := generateOperation(h, r, plainTok, manifests)
		ops[lm] = op
		for _, tag := range tagsOf(op) {
			usedTags[tag] = true
		}
	}
	for p := range openapiIgnored {
		if old, ok := oldPaths[p]; ok {
			newPaths[p] = old
		}
	}
	spec["paths"] = newPaths
	spec["tags"] = buildTags(spec, usedTags, manifests)
	ensureCommonComponents(spec)
	applySchemas(t, spec, routes)
	if info, ok := spec["info"].(map[string]any); ok {
		info["version"] = "2.0.0"
	}

	js, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(openapiJSON, append(js, '\n'), 0o644); err != nil { // #nosec G306 -- documentação pública
		t.Fatal(err)
	}
	var generic any
	_ = json.Unmarshal(js, &generic)
	ys, err := yaml.Marshal(generic)
	if err != nil {
		t.Fatal(err)
	}
	header := "# Gerado a partir de backend/docs/openapi.json — ver internal/app/openapi_test.go.\n"
	if err := os.WriteFile(filepath.Clean(openapiYAML), append([]byte(header), ys...), 0o644); err != nil { // #nosec G306
		t.Fatal(err)
	}
	t.Logf("OpenAPI regenerado: %d rotas", len(routes))
}

func tagsOf(op any) []string {
	m, _ := op.(map[string]any)
	var out []string
	if ts, ok := m["tags"].([]any); ok {
		for _, x := range ts {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func generateOperation(h *apiHarness, r routeInfo, plainTok string, manifests map[string]kernel.Manifest) map[string]any {
	probe := concrete(strings.NewReplacer("{key}", "blog", "{ref}", "x").Replace(r.path))
	anon := h.do(r.method, probe, "", "{}").Code
	public := anon != http.StatusUnauthorized
	needsPerm := false
	if !public {
		needsPerm = h.do(r.method, probe, plainTok, "{}").Code == http.StatusForbidden
	}

	tag := r.module
	if tag == "" {
		tag = coreTag(r.path)
	}
	summary := verbFor(r.method, r.path) + " " + strings.TrimPrefix(r.path, "/api/v1")
	if m, ok := manifests[r.module]; ok {
		summary = m.Name + " — " + summary
	}
	op := map[string]any{
		"tags":        []any{tag},
		"summary":     summary,
		"operationId": operationID(r.method, r.path),
	}
	var params []any
	for _, m := range chiParam.FindAllStringSubmatch(r.path, -1) {
		params = append(params, map[string]any{"name": m[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
	}
	if r.method == http.MethodPost || r.method == http.MethodPut || r.method == http.MethodPatch {
		params = append(params, map[string]any{"$ref": "#/components/parameters/IdempotencyKey"})
		op["requestBody"] = map[string]any{
			"required": false,
			"content":  map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object"}}},
		}
	}
	if len(params) > 0 {
		op["parameters"] = params
	}
	okCode := "200"
	switch {
	case r.method == http.MethodDelete:
		okCode = "204"
	case r.method == http.MethodPost && !strings.Contains(r.path, "/{"):
		okCode = "201"
	}
	responses := map[string]any{
		okCode: map[string]any{"$ref": "#/components/responses/Success"},
		"422":  map[string]any{"$ref": "#/components/responses/ValidationError"},
		"429":  map[string]any{"$ref": "#/components/responses/TooManyRequests"},
	}
	if okCode == "204" {
		responses[okCode] = map[string]any{"description": "Sem conteúdo."}
	}
	if !public {
		op["security"] = []any{map[string]any{"bearerAuth": []any{}}}
		responses["401"] = map[string]any{"$ref": "#/components/responses/Unauthorized"}
	} else {
		op["security"] = []any{}
	}
	if needsPerm {
		responses["403"] = map[string]any{"$ref": "#/components/responses/Forbidden"}
		op["description"] = "Exige uma permissão específica (recurso:ação) declarada no manifesto do módulo — ver GET /api/v1/iam/permissions."
	}
	if r.module != "" {
		responses["404"] = map[string]any{"$ref": "#/components/responses/ModuleDisabledOrNotFound"}
	}
	op["responses"] = responses
	return op
}

func coreTag(path string) string {
	p := strings.TrimPrefix(path, "/api/v1/")
	switch {
	case p == path:
		return "plataforma"
	case strings.HasPrefix(p, "system/"), strings.HasPrefix(p, "admin/modules"):
		return "kernel"
	case strings.HasPrefix(p, "admin/"):
		return "administracao"
	}
	seg, _, _ := strings.Cut(p, "/")
	return seg
}

func verbFor(method, path string) string {
	item := strings.HasSuffix(path, "}")
	switch method {
	case http.MethodGet:
		if item {
			return "Obter"
		}
		return "Listar/consultar"
	case http.MethodPost:
		if item || strings.Contains(path, "}/") {
			return "Executar ação"
		}
		return "Criar"
	case http.MethodPut:
		return "Salvar"
	case http.MethodPatch:
		return "Alterar"
	case http.MethodDelete:
		return "Excluir"
	}
	return method
}

var nonWord = regexp.MustCompile(`[^A-Za-z0-9]+`)

func operationID(method, path string) string {
	parts := nonWord.Split(strings.TrimPrefix(path, "/api/v1"), -1)
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func buildTags(spec map[string]any, used map[string]bool, manifests map[string]kernel.Manifest) []any {
	existing := map[string]map[string]any{}
	if ts, ok := spec["tags"].([]any); ok {
		for _, x := range ts {
			if m, ok := x.(map[string]any); ok {
				if n, ok := m["name"].(string); ok {
					existing[n] = m
				}
			}
		}
	}
	names := make([]string, 0, len(used))
	for n := range used {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]any, 0, len(names))
	for _, n := range names {
		if m, ok := manifests[n]; ok {
			kind := "Plug-in"
			if m.Core {
				kind = "Núcleo"
			}
			desc := kind + " — " + m.Description
			if len(m.DependsOn) > 0 {
				desc += " Depende de: " + strings.Join(m.DependsOn, ", ") + "."
			}
			out = append(out, map[string]any{"name": n, "description": m.Name + ". " + desc})
			continue
		}
		if t, ok := existing[n]; ok {
			out = append(out, t)
			continue
		}
		out = append(out, map[string]any{"name": n})
	}
	return out
}

func ensureCommonComponents(spec map[string]any) {
	comps, _ := spec["components"].(map[string]any)
	if comps == nil {
		comps = map[string]any{}
		spec["components"] = comps
	}
	responses, _ := comps["responses"].(map[string]any)
	if responses == nil {
		responses = map[string]any{}
		comps["responses"] = responses
	}
	envelope := map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Envelope"}}}
	defaults := map[string]any{
		"Success":                  map[string]any{"description": "Sucesso — payload em `data` (listas paginadas trazem `meta`).", "content": envelope},
		"ValidationError":          map[string]any{"description": "Dados inválidos (VALIDATION_ERROR) ou regra de negócio violada.", "content": envelope},
		"ModuleDisabledOrNotFound": map[string]any{"description": "Recurso inexistente ou módulo desativado no Kernel (MODULE_DISABLED).", "content": envelope},
	}
	for k, v := range defaults {
		if _, ok := responses[k]; !ok {
			responses[k] = v
		}
	}
}
