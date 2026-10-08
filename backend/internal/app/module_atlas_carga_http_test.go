package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type impactoCarga struct {
	Hash     string         `json:"hash"`
	Aplicada bool           `json:"aplicada"`
	Totais   map[string]int `json:"totais"`
	Series   []struct {
		Codigo   string `json:"codigo"`
		Situacao string `json:"situacao"`
		Antes    *struct {
			Observacoes string `json:"observacoes"`
		} `json:"antes"`
		Depois *struct {
			Observacoes string `json:"observacoes"`
		} `json:"depois"`
	} `json:"series"`
	Procedimentos []struct {
		CodigoProcessual string `json:"codigo_processual"`
		Situacao         string `json:"situacao"`
	} `json:"procedimentos"`
}

func corpoCarga(formato, conteudo, hash string) string {
	b, _ := json.Marshal(map[string]string{"formato": formato, "conteudo": conteudo, "hash": hash})
	return string(b)
}

// Atualização da TTDD pela tela (ADR 022): a planilha exportada, revisada e
// reenviada — simular mostra o impacto sem gravar; aplicar exige o mesmo
// arquivo (hash), grava com histórico e revogação e é auditado.
func TestAtlasCargaTTDDHTTP(t *testing.T) {
	h := newHarness(t)
	gestor := h.globalCom(t, "atlas:manage")
	_, comum := h.user("nexus-user")
	const simular, aplicar = "/api/v1/atlas/admin/ttdd/carga/simular", "/api/v1/atlas/admin/ttdd/carga/aplicar"

	original := strings.TrimPrefix(h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/exportar?codigo=2.0", "", "").Body.String(), string([]byte{0xEF, 0xBB, 0xBF}))
	t.Cleanup(func() { // volta à TTDD original, mesmo se o teste falhar no meio
		hash := data[impactoCarga](t, h.expect(http.StatusOK, http.MethodPost, simular, gestor, corpoCarga("csv", original, ""))).Hash
		h.expect(http.StatusOK, http.MethodPost, aplicar, gestor, corpoCarga("csv", original, hash))
	})

	h.expect(http.StatusUnauthorized, http.MethodPost, simular, "", corpoCarga("csv", original, ""))
	h.expect(http.StatusForbidden, http.MethodPost, simular, comum, corpoCarga("csv", original, ""))
	h.expect(http.StatusBadRequest, http.MethodPost, simular, gestor, `{`)
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, simular, gestor, `{"formato":"xlsx","conteudo":"x"}`)
	if body := h.expect(http.StatusUnprocessableEntity, http.MethodPost, simular, gestor, corpoCarga("csv", "Código;Outra\n", "")).Body.String(); !strings.Contains(body, "cabeçalho") {
		t.Fatalf("cabeçalho errado: %s", body)
	}

	// A mesma tabela de volta: nada muda.
	if sem := data[impactoCarga](t, h.expect(http.StatusOK, http.MethodPost, simular, gestor, corpoCarga("csv", original, ""))); len(sem.Series) != 0 ||
		sem.Totais["INALTERADA"] == 0 || sem.Aplicada {
		t.Fatalf("reenviar a mesma TTDD: %+v", sem)
	}

	// Revisão na planilha: observação nova na Dispensa (procedimento
	// ADM.DIR.002 aponta para ela) e uma série a menos.
	var linhas []string
	for _, l := range strings.Split(original, "\n") {
		switch {
		case strings.HasPrefix(l, "2.0.01.01.02;"): // retirada da tabela
		case strings.HasPrefix(l, "2.0.02.01.02;"):
			campos := strings.Split(l, ";")
			campos[8] = "Revisada pela CCPAD"
			linhas = append(linhas, strings.Join(campos, ";"))
		default:
			linhas = append(linhas, l)
		}
	}
	revisada := strings.Join(linhas, "\n")
	imp := data[impactoCarga](t, h.expect(http.StatusOK, http.MethodPost, simular, gestor, corpoCarga("csv", revisada, "")))
	situacao := map[string]string{}
	for _, s := range imp.Series {
		situacao[s.Codigo] = s.Situacao
		if s.Codigo == "2.0.02.01.02" && (s.Antes == nil || s.Depois == nil || s.Depois.Observacoes != "Revisada pela CCPAD") {
			t.Fatalf("antes/depois da alterada: %+v", s)
		}
	}
	afetado := false
	for _, p := range imp.Procedimentos {
		afetado = afetado || (p.CodigoProcessual == "ADM.DIR.002" && p.Situacao == "ALTERADA")
	}
	if imp.Totais["ALTERADA"] != 1 || imp.Totais["REVOGADA"] != 1 || situacao["2.0.02.01.02"] != "ALTERADA" ||
		situacao["2.0.01.01.02"] != "REVOGADA" || !afetado || len(imp.Hash) != 64 {
		t.Fatalf("impacto da revisão: %+v", imp)
	}
	// A simulação não gravou nada.
	if body := h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/2.0.01.01.02/historico", "", "").Body.String(); strings.Contains(body, "REVOGADA") {
		t.Fatalf("simular gravou: %s", body)
	}

	// Aplicar exige o arquivo simulado.
	if body := h.expect(http.StatusUnprocessableEntity, http.MethodPost, aplicar, gestor, corpoCarga("csv", revisada+"\n", imp.Hash)).Body.String(); !strings.Contains(body, "simule de novo") {
		t.Fatalf("hash diferente: %s", body)
	}
	if ok := data[impactoCarga](t, h.expect(http.StatusOK, http.MethodPost, aplicar, gestor, corpoCarga("csv", revisada, imp.Hash))); !ok.Aplicada {
		t.Fatalf("aplicar: %+v", ok)
	}
	rev := data[struct {
		RevogadaEm  *string `json:"revogada_em"`
		Observacoes string  `json:"observacoes"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/2.0.01.01.02", "", ""))
	alt := data[struct {
		Observacoes string `json:"observacoes"`
	}](t, h.expect(http.StatusOK, http.MethodGet, "/api/v1/atlas/ttdd/2.0.02.01.02", "", ""))
	if rev.RevogadaEm == nil || alt.Observacoes != "Revisada pela CCPAD" {
		t.Fatalf("gravação: %+v %+v", rev, alt)
	}
	var audits int
	if err := h.d.DB.QueryRow(t.Context(), `SELECT count(*) FROM audit_logs WHERE action = 'atlas.ttdd.carga.aplicada' AND resource_id = $1`,
		imp.Hash).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("auditoria da carga: %d %v", audits, err)
	}
	// O JSON gerado do PDF também é aceito (mesmo fluxo).
	h.expect(http.StatusUnprocessableEntity, http.MethodPost, simular, gestor, corpoCarga("json", "{", ""))
}
