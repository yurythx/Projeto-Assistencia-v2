package app

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/go-chi/chi/v5"
	"golang.org/x/tools/go/packages"
)

// Schemas do contrato derivados do código. Para cada rota, o handler
// montado (achado pelo nome da função em runtime) é analisado
// estaticamente: o tipo passado a httputil.Bind/DecodeJSON vira o corpo da
// requisição; o passado a WriteOK/WriteCreated/WriteAccepted/WritePage/
// WriteJSON vira o `data` da resposta; httputil.Query/Page/
// OptionalUUIDQuery e r.URL.Query().Get viram parâmetros de query. Os
// tipos Go viram components/schemas (tags json, ponteiro = nullable,
// validate: required/max/min/oneof, constantes do tipo = enum).
//
// Tudo o que é gerado leva "x-nexus-generated": na regeneração é refeito
// do zero; o que foi escrito à mão (sem a marca) nunca é tocado. O teste
// de contrato compara o arquivo com o resultado da geração: mudar um DTO
// sem regenerar quebra o CI.

const (
	modulePath  = "github.com/yurythx/projeto-nexus"
	httputilPkg = modulePath + "/pkg/httputil"
	genMark     = "x-nexus-generated"
)

// handlerName devolve o nome em runtime da função que atende a rota
// (desembrulhando as cadeias de middleware dos grupos do chi).
func handlerName(h http.Handler) string {
	for {
		c, ok := h.(*chi.ChainHandler)
		if !ok {
			break
		}
		h = c.Endpoint
	}
	if f, ok := h.(http.HandlerFunc); ok {
		return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	}
	t := reflect.TypeOf(h)
	star := ""
	if t.Kind() == reflect.Pointer {
		t, star = t.Elem(), "*"
	}
	if star != "" {
		return t.PkgPath() + ".(*" + t.Name() + ").ServeHTTP"
	}
	return t.PkgPath() + "." + t.Name() + ".ServeHTTP"
}

// ------------------------------------------------------------ análise

type funcSrc struct {
	decl *ast.FuncDecl
	pkg  *packages.Package
}

type schemaGen struct {
	funcs      map[string]funcSrc        // nome runtime → declaração
	components map[string]map[string]any // nome → schema gerado
	names      map[string]string         // tipo (pkg.Nome) → nome do componente
	taken      map[string]bool           // nomes de componentes escritos à mão
	hand       map[string]map[string]any // schemas escritos à mão (para reaproveitar)
	building   map[string]bool
	// input: gerando o corpo de uma requisição. required muda de sentido:
	// na resposta, o campo sempre presente no JSON (sem omitempty); na
	// requisição, o que o cliente precisa enviar (validate:"required").
	input bool
}

func loadSchemaGen(t *testing.T, hand map[string]map[string]any) *schemaGen {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedTypesInfo | packages.NeedImports,
		Dir: "../..",
	}
	pkgs, err := packages.Load(cfg, "./internal/...", "./pkg/...")
	if err != nil {
		t.Fatal(err)
	}
	g := &schemaGen{funcs: map[string]funcSrc{}, components: map[string]map[string]any{},
		names: map[string]string{}, taken: map[string]bool{}, hand: hand, building: map[string]bool{}}
	for name := range hand {
		g.taken[name] = true
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			t.Fatalf("pacote %s: %v", p.PkgPath, p.Errors)
		}
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if fn, ok := p.TypesInfo.Defs[fd.Name].(*types.Func); ok {
					g.funcs[runtimeName(fn)] = funcSrc{fd, p}
				}
			}
		}
	}
	return g
}

// runtimeName formata uma função como runtime.FuncForPC a nomeia.
func runtimeName(fn *types.Func) string {
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil {
		return fn.Pkg().Path() + "." + fn.Name()
	}
	rt := sig.Recv().Type()
	if p, ok := rt.(*types.Pointer); ok {
		return fn.Pkg().Path() + ".(*" + p.Elem().(*types.Named).Obj().Name() + ")." + fn.Name()
	}
	return fn.Pkg().Path() + "." + rt.(*types.Named).Obj().Name() + "." + fn.Name()
}

var closureSuffix = regexp.MustCompile(`((?:\.func\d+)(?:\.\d+)*)$`)

// resolve acha o corpo do handler: um método/função declarado, ou uma
// closure (outer.funcN.M: a N-ésima função literal de outer, e assim por
// diante).
func (g *schemaGen) resolve(name string) (ast.Node, *packages.Package, bool) {
	name = strings.TrimSuffix(name, "-fm")
	if body, pkg, ok := g.resolveExact(name); ok {
		return body, pkg, true
	}
	return g.resolveInlined(name)
}

// inlinedCallee acha, num nome como "pkg.Caller.func1.(*T).factory.3" ou
// "pkg.New.Factory.func5", a última função nomeada ("(*T).factory",
// "Factory"): a fábrica de handler que o compilador inlinou no chamador. A
// numeração da closure depende do inlining (e muda com -race/cobertura),
// então só o nome da fábrica é confiável.
var inlinedCallee = regexp.MustCompile(`\.((?:\(\*[A-Za-z_]\w*\)\.)?[A-Za-z_]\w*)(?:\.(?:func)?\d+)+$`)

func (g *schemaGen) resolveInlined(name string) (ast.Node, *packages.Package, bool) {
	m := inlinedCallee.FindStringSubmatch(name)
	if m == nil {
		return nil, nil, false
	}
	var found []funcSrc
	for key, src := range g.funcs {
		if strings.HasSuffix(key, "."+m[1]) {
			found = append(found, src)
		}
	}
	if len(found) != 1 {
		return nil, nil, false
	}
	if lit := returnedFuncLit(found[0].decl); lit != nil {
		return lit.Body, found[0].pkg, true
	}
	return nil, nil, false
}

// returnedFuncLit devolve a função literal que a fábrica retorna (direto
// ou convertida, ex.: http.HandlerFunc(func...)), se houver só uma.
func returnedFuncLit(fd *ast.FuncDecl) *ast.FuncLit {
	var lits []*ast.FuncLit
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, e := range ret.Results {
			if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
				e = c.Args[0]
			}
			if fl, ok := e.(*ast.FuncLit); ok {
				lits = append(lits, fl)
			}
		}
		return true
	})
	if len(lits) != 1 {
		return nil
	}
	return lits[0]
}

func (g *schemaGen) resolveExact(name string) (ast.Node, *packages.Package, bool) {
	var path []int
	if m := closureSuffix.FindString(name); m != "" {
		name = strings.TrimSuffix(name, m)
		for _, part := range strings.Split(strings.TrimPrefix(m, "."), ".") {
			n, _ := strconv.Atoi(strings.TrimPrefix(part, "func"))
			path = append(path, n)
		}
	}
	src, ok := g.funcs[name]
	if !ok {
		return nil, nil, false
	}
	var node ast.Node = src.decl.Body
	for _, n := range path {
		lits := directFuncLits(node)
		if n < 1 || n > len(lits) {
			return nil, nil, false
		}
		node = lits[n-1].Body
	}
	return node, src.pkg, true
}

// directFuncLits lista, em ordem de fonte, as funções literais de node que
// não estão dentro de outra função literal.
func directFuncLits(node ast.Node) []*ast.FuncLit {
	var out []*ast.FuncLit
	ast.Inspect(node, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok && n != node {
			out = append(out, fl)
			return false
		}
		return true
	})
	return out
}

type respInfo struct {
	data      types.Type
	meta      types.Type
	paged     bool
	noContent bool
}

type queryParam struct {
	name   string
	schema map[string]any
	ref    string
}

type opInfo struct {
	request   types.Type
	responses map[int][]respInfo
	query     []queryParam
}

// analyze percorre o handler (e as funções do módulo para as quais ele
// repassa o ResponseWriter) coletando corpo, respostas e query.
func (g *schemaGen) analyze(name string) (*opInfo, bool) {
	body, pkg, ok := g.resolve(name)
	if !ok {
		return nil, false
	}
	info := &opInfo{responses: map[int][]respInfo{}}
	g.walk(body, pkg, info, map[string]bool{name: true}, 0)
	return info, true
}

func (g *schemaGen) walk(node ast.Node, pkg *packages.Package, info *opInfo, seen map[string]bool, depth int) {
	ti := pkg.TypesInfo
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := calledFunc(ti, call)
		if fn == nil || fn.Pkg() == nil {
			return true
		}
		switch {
		case fn.Pkg().Path() == httputilPkg:
			g.httputilCall(ti, fn.Name(), call, info)
		case fn.Name() == "Get" && isURLValues(ti, call):
			if s, ok := constString(ti, call.Args[0]); ok {
				info.query = append(info.query, queryParam{name: s, schema: map[string]any{"type": "string"}})
			}
		case strings.HasPrefix(fn.Pkg().Path(), modulePath) && depth < 4 && passesHTTP(ti, call):
			key := runtimeName(fn.Origin())
			if seen[key] {
				return true
			}
			seen[key] = true
			if src, ok := g.funcs[key]; ok {
				g.walk(src.decl.Body, src.pkg, info, seen, depth+1)
			}
		}
		return true
	})
}

func (g *schemaGen) httputilCall(ti *types.Info, name string, call *ast.CallExpr, info *opInfo) {
	add := func(code int, r respInfo) {
		if r.data == nil && !r.noContent {
			return
		}
		for _, x := range info.responses[code] {
			if x.noContent == r.noContent && (r.noContent || types.Identical(x.data, r.data)) && x.paged == r.paged {
				return
			}
		}
		info.responses[code] = append(info.responses[code], r)
	}
	switch name {
	case "Bind", "DecodeJSON":
		if p, ok := ti.TypeOf(call.Args[2]).(*types.Pointer); ok {
			info.request = p.Elem()
		}
	case "WriteNoContent":
		add(http.StatusNoContent, respInfo{noContent: true})
	case "WriteOK":
		add(http.StatusOK, respInfo{data: argType(ti, call.Args[1])})
	case "WriteCreated":
		add(http.StatusCreated, respInfo{data: argType(ti, call.Args[1])})
	case "WriteAccepted":
		add(http.StatusAccepted, respInfo{data: argType(ti, call.Args[1])})
	case "WriteOKWithMeta":
		add(http.StatusOK, respInfo{data: argType(ti, call.Args[1]), meta: argType(ti, call.Args[2])})
	case "WritePage":
		add(http.StatusOK, respInfo{data: argType(ti, call.Args[1]), paged: true})
	case "WriteJSON":
		if tv, ok := ti.Types[call.Args[1]]; ok && tv.Value != nil {
			code, _ := constant.Int64Val(tv.Value)
			add(int(code), respInfo{data: argType(ti, call.Args[2]), meta: argType(ti, call.Args[3])})
		}
	case "Query":
		if s, ok := constString(ti, call.Args[1]); ok {
			sch := map[string]any{"type": "string"}
			if tv, ok := ti.Types[call.Args[2]]; ok && tv.Value != nil {
				max, _ := constant.Int64Val(tv.Value)
				sch["maxLength"] = max
			}
			info.query = append(info.query, queryParam{name: s, schema: sch})
		}
	case "OptionalUUIDQuery":
		if s, ok := constString(ti, call.Args[1]); ok {
			info.query = append(info.query, queryParam{name: s, schema: map[string]any{"type": "string", "format": "uuid"}})
		}
	case "Page":
		info.query = append(info.query, queryParam{name: "page", ref: "#/components/parameters/Page"},
			queryParam{name: "page_size", ref: "#/components/parameters/PageSize"})
	}
}

func calledFunc(ti *types.Info, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.IndexExpr: // função genérica instanciada
		if sel, ok := f.X.(*ast.SelectorExpr); ok {
			id = sel.Sel
		} else if i, ok := f.X.(*ast.Ident); ok {
			id = i
		}
	}
	if id == nil {
		return nil
	}
	fn, _ := ti.Uses[id].(*types.Func)
	return fn
}

func isURLValues(ti *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	n, ok := ti.TypeOf(sel.X).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "net/url" && n.Obj().Name() == "Values"
}

func passesHTTP(ti *types.Info, call *ast.CallExpr) bool {
	for _, a := range call.Args {
		t := ti.TypeOf(a)
		// *http.Request também: helpers que leem a query (ex.: filtros
		// compartilhados entre listagem e exportação).
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "net/http" &&
			(n.Obj().Name() == "ResponseWriter" || n.Obj().Name() == "Request") {
			return true
		}
	}
	return false
}

func constString(ti *types.Info, e ast.Expr) (string, bool) {
	tv, ok := ti.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// argType é o tipo estático do argumento (nil para o nil literal).
func argType(ti *types.Info, e ast.Expr) types.Type {
	t := ti.TypeOf(e)
	if b, ok := t.(*types.Basic); ok && b.Kind() == types.UntypedNil {
		return nil
	}
	return t
}

// ------------------------------------------------------------ tipos → schema

var wellKnown = map[string]map[string]any{
	"time.Time":                                     {"type": "string", "format": "date-time"},
	"time.Duration":                                 {"type": "integer", "description": "Duração em nanossegundos."},
	"github.com/google/uuid.UUID":                   {"type": "string", "format": "uuid"},
	"github.com/google/uuid.NullUUID":               {"type": "string", "format": "uuid", "nullable": true},
	"encoding/json.RawMessage":                      {},
	"net/netip.Addr":                                {"type": "string"},
	"net/netip.Prefix":                              {"type": "string"},
	"net.IP":                                        {"type": "string"},
	"github.com/jackc/pgx/v5/pgtype.Text":           {"type": "string", "nullable": true},
	modulePath + "/internal/domain/pagination.Meta": {"$ref": "#/components/schemas/PaginationMeta"},
}

func typeKey(n *types.Named) string {
	if n.Obj().Pkg() == nil {
		return n.Obj().Name()
	}
	return n.Obj().Pkg().Path() + "." + n.Obj().Name()
}

// schema converte um tipo Go no schema do JSON que encoding/json produz.
func (g *schemaGen) schema(t types.Type) map[string]any {
	switch tt := t.(type) {
	case *types.Alias:
		return g.schema(types.Unalias(tt))
	case *types.Pointer:
		return nullable(g.schema(tt.Elem()))
	case *types.Named:
		if s, ok := wellKnown[typeKey(tt)]; ok {
			return clone(s)
		}
		if implements(tt, "MarshalJSON") {
			return map[string]any{"description": "Formato próprio (MarshalJSON)."}
		}
		if implements(tt, "MarshalText") {
			return map[string]any{"type": "string"}
		}
		if st, ok := tt.Underlying().(*types.Struct); ok {
			return map[string]any{"$ref": "#/components/schemas/" + g.component(tt, st)}
		}
		s := g.schema(tt.Underlying())
		if enum := enumOf(tt); len(enum) > 0 {
			s["enum"] = enum
		}
		return s
	case *types.Basic:
		switch {
		case tt.Info()&types.IsBoolean != 0:
			return map[string]any{"type": "boolean"}
		case tt.Info()&types.IsInteger != 0:
			return map[string]any{"type": "integer"}
		case tt.Info()&types.IsFloat != 0:
			return map[string]any{"type": "number"}
		case tt.Info()&types.IsString != 0:
			return map[string]any{"type": "string"}
		}
		return map[string]any{}
	case *types.Slice:
		if b, ok := tt.Elem().(*types.Basic); ok && b.Kind() == types.Byte {
			return map[string]any{"type": "string", "format": "byte"}
		}
		return map[string]any{"type": "array", "items": g.schema(tt.Elem())}
	case *types.Array:
		return map[string]any{"type": "array", "items": g.schema(tt.Elem())}
	case *types.Map:
		return map[string]any{"type": "object", "additionalProperties": g.schema(tt.Elem())}
	case *types.Struct:
		return g.structSchema(tt)
	}
	return map[string]any{} // interface, any: qualquer valor JSON
}

func nullable(s map[string]any) map[string]any {
	if _, ok := s["$ref"]; ok {
		return map[string]any{"allOf": []any{s}, "nullable": true}
	}
	s["nullable"] = true
	return s
}

func clone(s map[string]any) map[string]any {
	out := make(map[string]any, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func implements(n *types.Named, method string) bool {
	for _, t := range []types.Type{n, types.NewPointer(n)} {
		ms := types.NewMethodSet(t)
		for i := 0; i < ms.Len(); i++ {
			if ms.At(i).Obj().Name() == method {
				return true
			}
		}
	}
	return false
}

// enumOf lista as constantes do pacote declaradas com o tipo nomeado.
func enumOf(n *types.Named) []any {
	pkg := n.Obj().Pkg()
	if pkg == nil {
		return nil
	}
	var vals []string
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok || !types.Identical(c.Type(), n) || c.Val().Kind() != constant.String {
			continue
		}
		vals = append(vals, constant.StringVal(c.Val()))
	}
	sort.Strings(vals)
	vals = uniqStrings(vals)
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}

func uniqStrings(in []string) []string {
	var out []string
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// component registra o struct nomeado em components/schemas (o nome leva o
// módulo: blog.Post → BlogPost, transport.postRequest do blog →
// BlogPostRequest).
func (g *schemaGen) component(n *types.Named, st *types.Struct) string {
	key := typeKey(n)
	if g.input {
		key += "|input"
	}
	if name, ok := g.names[key]; ok {
		return name
	}
	base := componentName(n)
	// Um struct usado nos dois sentidos (ex.: branding.Settings) só ganha
	// uma variante "Input" se o required da requisição for diferente.
	if respName, ok := g.names[typeKey(n)]; ok && g.input {
		base = respName + "Input"
	}
	name := base
	for i := 2; g.taken[name] || g.nameUsed(name); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	g.names[key] = name
	g.building[key] = true
	s := g.structSchema(st)
	delete(g.building, key)
	if g.input {
		if respName, ok := g.names[typeKey(n)]; ok && sameSchema(g.components[respName], s) {
			g.names[key] = respName
			return respName
		}
	}
	// Um componente escrito à mão com exatamente a mesma forma (campos,
	// tipos, formatos) é reaproveitado: mantém as descrições e não duplica.
	if twin := g.handTwin(s); twin != "" {
		g.names[key] = twin
		return twin
	}
	s[genMark] = true
	g.components[name] = s
	return name
}

func sameSchema(a, b map[string]any) bool {
	if a == nil {
		return false
	}
	strip := func(m map[string]any) string {
		c := clone(m)
		delete(c, genMark)
		j, _ := json.Marshal(c)
		return string(j)
	}
	return strip(a) == strip(b)
}

func (g *schemaGen) handTwin(s map[string]any) string {
	names := make([]string, 0, len(g.hand))
	for n := range g.hand {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if refines(g.hand[n], s) {
			return n
		}
	}
	return ""
}

// refinement são as chaves que a versão escrita à mão pode acrescentar a um
// campo: restrições que o tipo Go não expressa (um string que só aceita
// alguns valores, um string que é data).
var refinement = map[string]bool{"enum": true, "format": true, "pattern": true, "minLength": true, "maxLength": true, "minimum": true, "maximum": true}

func isDocKey(k string) bool {
	return k == "description" || k == "example" || k == "required" || k == "title" || strings.HasPrefix(k, "x-")
}

// refines diz se o schema escrito à mão tem a mesma forma do gerado (mesmos
// campos, mesmos tipos), podendo só acrescentar restrições. O que é só
// documentação (description, example, required, x-*) não conta.
func refines(hand, gen map[string]any) bool {
	for k := range hand {
		if _, ok := gen[k]; !ok && !isDocKey(k) && !refinement[k] {
			return false
		}
	}
	for k, gv := range gen {
		if isDocKey(k) {
			continue
		}
		hv, ok := hand[k]
		if !ok {
			return false
		}
		switch k {
		case "properties":
			hp, _ := hv.(map[string]any)
			gp, _ := gv.(map[string]any)
			if len(hp) != len(gp) {
				return false
			}
			for name, sub := range gp {
				h, ok := hp[name].(map[string]any)
				gm, _ := sub.(map[string]any)
				if !ok || !refines(h, gm) {
					return false
				}
			}
		case "items", "additionalProperties":
			hm, _ := hv.(map[string]any)
			gm, _ := gv.(map[string]any)
			if hm == nil || gm == nil || !refines(hm, gm) {
				return false
			}
		case "enum":
			if !sameSet(hv, gv) {
				return false
			}
		default:
			a, _ := json.Marshal(hv)
			b, _ := json.Marshal(gv)
			if string(a) != string(b) {
				return false
			}
		}
	}
	return true
}

func sameSet(a, b any) bool {
	key := func(v any) string {
		xs, _ := v.([]any)
		out := make([]string, len(xs))
		for i, x := range xs {
			j, _ := json.Marshal(x)
			out[i] = string(j)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	return key(a) == key(b)
}

func (g *schemaGen) nameUsed(name string) bool {
	_, ok := g.components[name]
	if ok {
		return true
	}
	for _, v := range g.names {
		if v == name {
			return true
		}
	}
	return false
}

func componentName(n *types.Named) string {
	pkgPath := ""
	if n.Obj().Pkg() != nil {
		pkgPath = n.Obj().Pkg().Path()
	}
	scope := ""
	for _, root := range []string{"/internal/modules/", "/internal/platform/", "/internal/domain/", "/pkg/"} {
		if i := strings.Index(pkgPath, root); i >= 0 {
			scope, _, _ = strings.Cut(pkgPath[i+len(root):], "/")
			break
		}
	}
	scope = camel(scope)
	base := camel(n.Obj().Name())
	if strings.HasPrefix(base, scope) {
		return base
	}
	return scope + base
}

func camel(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if r == '_' || r == '-' || r == '.' {
			up = true
			continue
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

var validateMax = regexp.MustCompile(`^(max|min|len)=(\d+)$`)

// structSchema descreve os campos que encoding/json serializa. required: na
// resposta, os campos sem omitempty (sempre presentes no JSON); na
// requisição, os validate:"required".
func (g *schemaGen) structSchema(st *types.Struct) map[string]any {
	props := map[string]any{}
	var required []string
	validated := false
	for i := 0; i < st.NumFields(); i++ {
		if _, ok := reflect.StructTag(st.Tag(i)).Lookup("validate"); ok {
			validated = true
		}
	}
	g.fields(st, props, &required, validated)
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		req := make([]any, len(required))
		for i, r := range required {
			req[i] = r
		}
		s["required"] = req
	}
	return s
}

func (g *schemaGen) fields(st *types.Struct, props map[string]any, required *[]string, validated bool) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))
		jsonTag, hasTag := tag.Lookup("json")
		name, opts, _ := strings.Cut(jsonTag, ",")
		if name == "-" && opts == "" {
			continue
		}
		if f.Embedded() && (!hasTag || name == "") {
			et := f.Type()
			if p, ok := et.(*types.Pointer); ok {
				et = p.Elem()
			}
			if es, ok := et.Underlying().(*types.Struct); ok {
				g.fields(es, props, required, validated)
				continue
			}
		}
		if !f.Exported() {
			continue
		}
		if _, isFunc := f.Type().Underlying().(*types.Signature); isFunc {
			continue
		}
		if _, isChan := f.Type().Underlying().(*types.Chan); isChan {
			continue
		}
		if name == "" {
			name = f.Name()
		}
		omit := strings.Contains(","+opts+",", ",omitempty,") || strings.Contains(","+opts+",", ",omitzero,")
		fs := g.schema(f.Type())
		// ponteiro com omitempty nunca sai como null: nil é omitido
		if p, ok := f.Type().(*types.Pointer); ok && omit {
			fs = g.schema(p.Elem())
		}
		if strings.Contains(","+opts+",", ",string,") {
			fs = map[string]any{"type": "string"}
		}
		isRequired := false
		if v, ok := tag.Lookup("validate"); ok {
			isRequired = applyValidate(fs, v)
		}
		switch {
		case g.input && validated:
			if isRequired {
				*required = append(*required, name)
			}
		case g.input:
			// requisição sem validate: nada é exigido
		case !omit:
			*required = append(*required, name)
		}
		props[name] = fs
	}
}

// applyValidate traduz as regras do go-playground/validator que têm
// equivalente no schema; devolve se o campo é obrigatório.
func applyValidate(s map[string]any, rules string) bool {
	required := false
	kind, _ := s["type"].(string)
	for _, rule := range strings.Split(rules, ",") {
		if rule == "dive" {
			break // o resto vale para os itens
		}
		switch {
		case rule == "required":
			required = true
		case rule == "email":
			s["format"] = "email"
		case rule == "uuid", rule == "uuid4":
			s["format"] = "uuid"
		case rule == "url", rule == "http_url":
			s["format"] = "uri"
		case strings.HasPrefix(rule, "oneof="):
			var enum []any
			for _, v := range strings.Fields(strings.TrimPrefix(rule, "oneof=")) {
				if kind == "integer" {
					n, _ := strconv.Atoi(v)
					enum = append(enum, n)
				} else {
					enum = append(enum, v)
				}
			}
			s["enum"] = enum
		default:
			m := validateMax.FindStringSubmatch(rule)
			if m == nil {
				continue
			}
			n, _ := strconv.Atoi(m[2])
			limits := map[string][2]string{
				"string":  {"minLength", "maxLength"},
				"array":   {"minItems", "maxItems"},
				"integer": {"minimum", "maximum"},
				"number":  {"minimum", "maximum"},
			}[kind]
			if limits[0] == "" {
				continue
			}
			switch m[1] {
			case "min":
				s[limits[0]] = n
			case "max":
				s[limits[1]] = n
			case "len":
				s[limits[0]], s[limits[1]] = n, n
			}
		}
	}
	return required
}

// ------------------------------------------------------------ aplicação no contrato

// applySchemas reescreve, no contrato, tudo o que é gerado a partir do
// código. Idempotente: o que já estava gerado é descartado e refeito.
func applySchemas(t *testing.T, spec map[string]any, routes []routeInfo) {
	t.Helper()
	comps := spec["components"].(map[string]any)
	schemas, _ := comps["schemas"].(map[string]any)
	if schemas == nil {
		schemas = map[string]any{}
		comps["schemas"] = schemas
	}
	hand := map[string]map[string]any{}
	for name, s := range schemas {
		if m, ok := s.(map[string]any); ok && m[genMark] == true {
			delete(schemas, name)
			continue
		}
		if m, ok := s.(map[string]any); ok {
			hand[name] = m
		}
	}
	usedBefore := referencedSchemas(spec)
	g := loadSchemaGen(t, hand)
	paths := spec["paths"].(map[string]any)
	var unresolved []string
	var pending []func()
	for _, r := range routes {
		ops, _ := paths[r.path].(map[string]any)
		op, _ := ops[strings.ToLower(r.method)].(map[string]any)
		if op == nil || r.handler == "" {
			continue
		}
		info, ok := g.analyze(r.handler)
		if !ok {
			unresolved = append(unresolved, r.method+" "+r.path+" ("+r.handler+")")
			continue
		}
		g.applyOperation(op, info)
		pending = append(pending, func() { g.applyRequest(op, info) })
	}
	// Corpos de requisição por último: um struct usado nos dois sentidos
	// reaproveita o componente da resposta quando a forma coincide.
	g.input = true
	for _, apply := range pending {
		apply()
	}
	g.input = false
	for name, s := range g.components {
		schemas[name] = s
	}
	// Componente escrito à mão que só existia para as respostas agora
	// derivadas do código fica órfão: sai (os que já eram documentação
	// avulsa, sem referência, como EventEnvelope, ficam).
	usedAfter := referencedSchemas(spec)
	for name := range hand {
		if usedBefore[name] && !usedAfter[name] {
			delete(schemas, name)
		}
	}
	if len(unresolved) > 0 {
		t.Logf("handlers sem código-fonte analisável (mantidos genéricos): %v", unresolved)
	}
}

var schemaRef = regexp.MustCompile(`"#/components/schemas/([^"]+)"`)

// referencedSchemas lista os schemas referenciados fora da própria
// definição (caminhos, respostas, parâmetros e outros schemas).
func referencedSchemas(spec map[string]any) map[string]bool {
	out := map[string]bool{}
	mark := func(v any, self string) {
		b, _ := json.Marshal(v)
		for _, m := range schemaRef.FindAllStringSubmatch(string(b), -1) {
			if m[1] != self {
				out[m[1]] = true
			}
		}
	}
	mark(spec["paths"], "")
	comps, _ := spec["components"].(map[string]any)
	for k, v := range comps {
		if k != "schemas" {
			mark(v, "")
		}
	}
	schemas, _ := comps["schemas"].(map[string]any)
	for name, v := range schemas {
		mark(v, name)
	}
	return out
}

func isGenerated(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m[genMark] == true
}

func isGenericSuccess(v any) bool {
	m, ok := v.(map[string]any)
	return ok && len(m) == 1 && m["$ref"] == "#/components/responses/Success"
}

// defaultDescriptions são os textos que o próprio gerador escreve.
var defaultDescriptions = map[string]bool{
	"":              true,
	descSuccess:     true,
	descPage:        true,
	"Sem conteúdo.": true,
}

const (
	descSuccess = "Sucesso — payload em `data`."
	descPage    = "Página de itens em `data`, paginação em `meta`."
)

// typedResponses: toda resposta detectada é 204 ou tem um struct nomeado
// (direto, por ponteiro ou em lista) como data.
func typedResponses(byCode map[int][]respInfo) bool {
	for _, rs := range byCode {
		for _, r := range rs {
			if !r.noContent && !isStructType(r.data) {
				return false
			}
		}
	}
	return true
}

func isStructType(t types.Type) bool {
	for {
		switch tt := types.Unalias(t).(type) {
		case *types.Pointer:
			t = tt.Elem()
			continue
		case *types.Slice:
			t = tt.Elem()
			continue
		case *types.Named:
			if _, known := wellKnown[typeKey(tt)]; known || implements(tt, "MarshalJSON") {
				return false
			}
			_, ok := tt.Underlying().(*types.Struct)
			return ok
		}
		return false
	}
}

// onlyJSON: a resposta escrita à mão não documenta outro formato (CSV, XML).
func onlyJSON(r any) bool {
	content, _ := r.(map[string]any)["content"].(map[string]any)
	for ct := range content {
		if ct != "application/json" {
			return false
		}
	}
	return true
}

// isPlainNoContent é o 204 do esqueleto gerado para DELETE.
func isPlainNoContent(v any) bool {
	m, ok := v.(map[string]any)
	return ok && len(m) == 1 && m["description"] == "Sem conteúdo."
}

// applyRequest escreve o corpo da requisição.
func (g *schemaGen) applyRequest(op map[string]any, info *opInfo) {
	// corpo da requisição (nenhum handler lê o corpo fora de Bind/
	// DecodeJSON: sem a chamada, a operação não tem corpo)
	if _, ok := op["requestBody"]; !ok && info.request != nil {
		op["requestBody"] = map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object"}}}}
	}
	if rb, ok := op["requestBody"].(map[string]any); ok {
		content, _ := rb["content"].(map[string]any)
		js, _ := content["application/json"].(map[string]any)
		generic := false
		if js != nil {
			sch, _ := js["schema"].(map[string]any)
			generic = len(sch) == 1 && sch["type"] == "object"
		}
		switch {
		case !generic && !isGenerated(rb):
		case info.request == nil:
			delete(op, "requestBody")
		default:
			op["requestBody"] = map[string]any{
				"required": true,
				"content":  map[string]any{"application/json": map[string]any{"schema": g.schema(info.request)}},
				genMark:    true,
			}
		}
	}
}

// applyOperation escreve as respostas de sucesso e os parâmetros de query.
func (g *schemaGen) applyOperation(op map[string]any, info *opInfo) {
	// respostas de sucesso
	responses, _ := op["responses"].(map[string]any)
	if responses != nil && len(info.responses) > 0 {
		// Resposta escrita à mão só prevalece onde o código não tem tipo
		// (map, any) ou negocia outro formato além de JSON: com um struct,
		// o código é a fonte da verdade e o texto à mão ficaria defasado
		// (GET /me documentava um usuário do banco, não a identidade). A
		// description escrita à mão é mantida.
		typed := typedResponses(info.responses)
		replaceable := true
		descriptions := map[string]any{}
		for code, r := range responses {
			if !strings.HasPrefix(code, "2") || isGenericSuccess(r) || isPlainNoContent(r) {
				continue
			}
			if !isGenerated(r) && (!typed || !onlyJSON(r)) {
				replaceable = false
				continue
			}
			// a description escrita à mão sobrevive às regenerações
			if d, _ := r.(map[string]any)["description"].(string); !defaultDescriptions[d] {
				descriptions[code] = d
			}
		}
		if replaceable {
			for code, r := range responses {
				if strings.HasPrefix(code, "2") || isGenericSuccess(r) || isGenerated(r) || isPlainNoContent(r) {
					delete(responses, code)
				}
			}
			for code, rs := range info.responses {
				resp := g.response(rs)
				if d, ok := descriptions[strconv.Itoa(code)].(string); ok && d != "" {
					resp["description"] = d
				}
				responses[strconv.Itoa(code)] = resp
			}
		}
	}

	// parâmetros de query
	var params []any
	present := map[string]bool{}
	if ps, ok := op["parameters"].([]any); ok {
		for _, p := range ps {
			if isGenerated(p) {
				continue
			}
			params = append(params, p)
			if m, ok := p.(map[string]any); ok {
				if ref, ok := m["$ref"].(string); ok {
					present["ref:"+ref] = true
				}
				if n, ok := m["name"].(string); ok && m["in"] == "query" {
					present[n] = true
				}
			}
		}
	}
	for _, q := range info.query {
		if present[q.name] || (q.ref != "" && present["ref:"+q.ref]) {
			continue
		}
		present[q.name] = true
		if q.ref != "" {
			present["ref:"+q.ref] = true
			params = append(params, map[string]any{"$ref": q.ref})
			continue
		}
		params = append(params, map[string]any{"name": q.name, "in": "query", "required": false, "schema": q.schema, genMark: true})
	}
	if len(params) > 0 {
		op["parameters"] = params
	}
}

func (g *schemaGen) response(rs []respInfo) map[string]any {
	if rs[0].noContent {
		return map[string]any{"description": "Sem conteúdo.", genMark: true}
	}
	var variants []any
	desc := descSuccess
	for _, r := range rs {
		props := map[string]any{"data": g.schema(r.data)}
		switch {
		case r.paged:
			props["meta"] = map[string]any{"$ref": "#/components/schemas/PaginationMeta"}
			desc = descPage
		case r.meta != nil:
			props["meta"] = g.schema(r.meta)
		}
		variants = append(variants, map[string]any{"type": "object", "properties": props})
	}
	sort.Slice(variants, func(i, j int) bool {
		a, _ := json.Marshal(variants[i])
		b, _ := json.Marshal(variants[j])
		return string(a) < string(b)
	})
	var body any = variants[0]
	if len(variants) > 1 {
		body = map[string]any{"oneOf": variants}
	}
	return map[string]any{
		"description": desc,
		"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{
			"allOf": []any{map[string]any{"$ref": "#/components/schemas/Envelope"}, body},
		}}},
		genMark: true,
	}
}
