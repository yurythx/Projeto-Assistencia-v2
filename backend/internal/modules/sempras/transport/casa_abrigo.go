package transport

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/pkg/httputil"
)

// rotasCasaAbrigo monta as rotas da Casa Abrigo (ADR 032) — todas exigem
// sempras:atendimento; o serviço confere a unidade.
func (h *Handlers) rotasCasaAbrigo(r chi.Router, atendimento func(http.Handler) http.Handler) {
	a := r.With(atendimento)
	a.Get("/sempras/unidades/{id}/casa-abrigo", h.PainelCasaAbrigo)
	a.Get("/sempras/unidades/{id}/casa-abrigo/estatisticas", h.EstatisticasCasaAbrigo)
	a.Get("/sempras/unidades/{id}/casa-abrigo/evolucoes", h.EvolucoesCasaAbrigo)
	a.Get("/sempras/unidades/{id}/casa-abrigo/visitantes", h.VisitantesCasaAbrigo)
	a.Post("/sempras/unidades/{id}/casa-abrigo/visitantes", h.RegistrarVisita)
	a.Get("/sempras/unidades/{id}/casa-abrigo/termos", h.TermosCasaAbrigo)
	a.Get("/sempras/unidades/{id}/casa-abrigo/diario", h.DiarioCasaAbrigo)
	a.Post("/sempras/unidades/{id}/casa-abrigo/diario", h.RegistrarAnotacaoDiario)
	a.Get("/sempras/unidades/{id}/casa-abrigo/relatorios", h.RelatoriosCasaAbrigo)
	a.Post("/sempras/unidades/{id}/casa-abrigo/relatorios", h.RegistrarRelatorio)
	a.Get("/sempras/unidades/{id}/casa-abrigo/tarefas", h.TarefasCasaAbrigo)
	a.Post("/sempras/unidades/{id}/casa-abrigo/tarefas", h.CriarTarefa)

	a.Post("/sempras/cidadaos/{id}/casa-abrigo", h.RegistrarAcolhimento)
	a.Get("/sempras/casa-abrigo/{id}", h.ObterAcolhimento)
	a.Put("/sempras/casa-abrigo/{id}", h.AtualizarAcolhimento)
	a.Post("/sempras/casa-abrigo/{id}/desligar", h.DesligarAcolhimento)
	a.Post("/sempras/casa-abrigo/{id}/irmaos", h.AdicionarIrmao)
	a.Delete("/sempras/casa-abrigo-irmaos/{id}", h.RemoverIrmao)
	a.Post("/sempras/casa-abrigo/{id}/diagnosticos", h.AdicionarDiagnostico)
	a.Delete("/sempras/casa-abrigo-diagnosticos/{id}", h.RemoverDiagnostico)
	a.Post("/sempras/casa-abrigo/{id}/visitantes", h.AdicionarVisitante)
	a.Post("/sempras/casa-abrigo-visitantes/{id}/saida", h.RegistrarSaidaVisitante)
	a.Delete("/sempras/casa-abrigo-visitantes/{id}", h.RemoverVisitante)
	a.Post("/sempras/casa-abrigo/{id}/termos", h.RegistrarTermo)
	a.Delete("/sempras/casa-abrigo-termos/{id}", h.RemoverTermo)
	a.Put("/sempras/casa-abrigo/{id}/pia", h.SalvarPiaFicha)
	a.Post("/sempras/casa-abrigo/{id}/pia-metas", h.AdicionarPiaMeta)
	a.Put("/sempras/casa-abrigo-pia-metas/{id}", h.EditarPiaMeta)
	a.Post("/sempras/casa-abrigo-pia-metas/{id}/concluir", h.ConcluirPiaMeta)
	a.Delete("/sempras/casa-abrigo-pia-metas/{id}", h.RemoverPiaMeta)
	a.Post("/sempras/casa-abrigo/{id}/documentos/uploads", h.StartUploadDocumento)
	a.Post("/sempras/casa-abrigo-documentos/{id}/confirm", h.ConfirmUploadDocumento)
	a.Get("/sempras/casa-abrigo-documentos/{id}/download", h.DownloadDocumento)
	a.Delete("/sempras/casa-abrigo-documentos/{id}", h.RemoverDocumento)
	a.Put("/sempras/casa-abrigo-relatorios/{id}", h.AtualizarRelatorio)
	a.Delete("/sempras/casa-abrigo-relatorios/{id}", h.RemoverRelatorio)
	a.Put("/sempras/casa-abrigo-tarefas/{id}", h.AtualizarTarefa)
	a.Delete("/sempras/casa-abrigo-tarefas/{id}", h.RemoverTarefa)
}

// ------------------------------------------------------------ acolhimento

// IrmaoRequest é um irmão do acolhido.
type IrmaoRequest struct {
	Nome             string     `json:"nome" validate:"required,min=2,max=200"`
	DataNascimento   *time.Time `json:"data_nascimento"`
	EstaAcolhido     bool       `json:"esta_acolhido"`
	UnidadeDescricao string     `json:"unidade_descricao" validate:"max=200"`
	Observacao       string     `json:"observacao" validate:"max=1000"`
}

func (req IrmaoRequest) input() application.IrmaoInput {
	return application.IrmaoInput{Nome: req.Nome, DataNascimento: req.DataNascimento, EstaAcolhido: req.EstaAcolhido,
		UnidadeDescricao: req.UnidadeDescricao, Observacao: req.Observacao}
}

// DiagnosticoRequest é um diagnóstico de saúde/deficiência/transtorno do acolhido.
type DiagnosticoRequest struct {
	Categoria   string `json:"categoria" validate:"required,oneof=doenca deficiencia transtorno outro"`
	Diagnostico string `json:"diagnostico" validate:"required,min=2,max=255"`
	Descricao   string `json:"descricao" validate:"max=2000"`
}

func (req DiagnosticoRequest) input() application.DiagnosticoInput {
	return application.DiagnosticoInput{Categoria: req.Categoria, Diagnostico: req.Diagnostico, Descricao: req.Descricao}
}

// AcolhimentoRequest é o assistente de 6 etapas (edição: sem irmãos e
// diagnósticos, que têm ações próprias; nunca o desligamento).
type AcolhimentoRequest struct {
	domain.AcolhimentoDados
	DataEntrada time.Time `json:"data_entrada" validate:"required"`
}

// NovoAcolhimentoRequest é o cadastro: a unidade, o assistente e os
// irmãos e diagnósticos marcados (gravados na mesma transação).
type NovoAcolhimentoRequest struct {
	AcolhimentoRequest
	UnidadeID    uuid.UUID            `json:"unidade_id" validate:"required"`
	Irmaos       []IrmaoRequest       `json:"irmaos" validate:"max=30,dive"`
	Diagnosticos []DiagnosticoRequest `json:"diagnosticos" validate:"max=60,dive"`
}

// RegistrarAcolhimento abre um acolhimento institucional na Casa Abrigo.
func (h *Handlers) RegistrarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req NovoAcolhimentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	in := application.AcolhimentoInput{Dados: req.AcolhimentoDados, DataEntrada: req.DataEntrada}
	for _, i := range req.Irmaos {
		in.Irmaos = append(in.Irmaos, i.input())
	}
	for _, d := range req.Diagnosticos {
		in.Diagnosticos = append(in.Diagnosticos, d.input())
	}
	a, err := h.svc.RegistrarAcolhimento(r.Context(), identity(r), id, req.UnidadeID, in)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, a)
}

// ObterAcolhimento devolve o acolhimento completo (leitura auditada).
func (h *Handlers) ObterAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.ObterAcolhimento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, a)
}

// AtualizarAcolhimento regrava o assistente e a data de entrada.
func (h *Handlers) AtualizarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AcolhimentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.AtualizarAcolhimento(r.Context(), identity(r), id, req.AcolhimentoDados, req.DataEntrada)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, a)
}

// DesligamentoRequest é o desligamento de um acolhimento (R5).
type DesligamentoRequest struct {
	Tipo      string     `json:"tipo" validate:"required,max=40"`
	Motivo    string     `json:"motivo" validate:"required,min=3,max=2000"`
	DataSaida time.Time  `json:"data_saida" validate:"required"`
	Guia      string     `json:"guia" validate:"max=200"`
	DataGuia  *time.Time `json:"data_guia"`
}

// DesligarAcolhimento encerra o acolhimento (equipe técnica).
func (h *Handlers) DesligarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DesligamentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.DesligarAcolhimento(r.Context(), identity(r), id, domain.Desligamento{Tipo: req.Tipo, Motivo: req.Motivo,
		Guia: req.Guia, DataSaida: req.DataSaida, DataGuia: req.DataGuia})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, a)
}

// AdicionarIrmao registra um irmão do acolhido.
func (h *Handlers) AdicionarIrmao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req IrmaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	i, err := h.svc.AdicionarIrmao(r.Context(), identity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, i)
}

// RemoverIrmao remove um irmão registrado por engano.
func (h *Handlers) RemoverIrmao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverIrmao(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// AdicionarDiagnostico registra um diagnóstico do acolhido.
func (h *Handlers) AdicionarDiagnostico(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DiagnosticoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.AdicionarDiagnostico(r.Context(), identity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, d)
}

// RemoverDiagnostico remove um diagnóstico (desmarcado no checklist).
func (h *Handlers) RemoverDiagnostico(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverDiagnostico(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// ------------------------------------------------------------ visitantes

// VisitanteRequest é uma visita recebida pela unidade. Pela ficha do
// acolhido, tipo e acolhido vêm da rota.
type VisitanteRequest struct {
	AcolhimentoID       *uuid.UUID `json:"acolhimento_id"`
	TipoVisita          string     `json:"tipo_visita" validate:"omitempty,oneof=acolhido instituicao doacao voluntariado"`
	NomeVisitante       string     `json:"nome_visitante" validate:"required,min=2,max=200"`
	DocumentoIdentidade string     `json:"documento_identidade" validate:"max=30"`
	ParentescoVinculo   string     `json:"parentesco_vinculo" validate:"max=100"`
	DataVisita          time.Time  `json:"data_visita" validate:"required"`
	HoraEntrada         string     `json:"hora_entrada" validate:"max=5"`
	HoraSaida           string     `json:"hora_saida" validate:"max=5"`
	Observacoes         string     `json:"observacoes" validate:"max=1000"`
}

func (req VisitanteRequest) dados() domain.VisitanteDados {
	return domain.VisitanteDados{AcolhimentoID: req.AcolhimentoID, TipoVisita: req.TipoVisita, NomeVisitante: req.NomeVisitante,
		DocumentoIdentidade: req.DocumentoIdentidade, ParentescoVinculo: req.ParentescoVinculo, DataVisita: req.DataVisita,
		HoraEntrada: req.HoraEntrada, HoraSaida: req.HoraSaida, Observacoes: req.Observacoes}
}

// AdicionarVisitante registra uma visita a um acolhido (pela ficha).
func (h *Handlers) AdicionarVisitante(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req VisitanteRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.AdicionarVisitante(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, v)
}

// RegistrarVisita registra a entrada de um visitante na unidade.
func (h *Handlers) RegistrarVisita(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req VisitanteRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.RegistrarVisita(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, v)
}

// SaidaVisitanteRequest é a hora de saída do visitante.
type SaidaVisitanteRequest struct {
	HoraSaida string `json:"hora_saida" validate:"required,max=5"`
}

// RegistrarSaidaVisitante anota a hora de saída do visitante.
func (h *Handlers) RegistrarSaidaVisitante(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req SaidaVisitanteRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.RegistrarSaidaVisitante(r.Context(), identity(r), id, req.HoraSaida)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, v)
}

// RemoverVisitante remove uma visita registrada por engano.
func (h *Handlers) RemoverVisitante(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverVisitante(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// VisitantesCasaAbrigo lista as visitas recentes da unidade.
func (h *Handlers) VisitantesCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.VisitantesDaUnidade(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ---------------------------------------------------------------- termos

// TermoRequest é um termo de advertência, passeio ou saída.
type TermoRequest struct {
	TipoTermo             string    `json:"tipo_termo" validate:"required,oneof=advertencia passeio saida outro"`
	ResponsavelRetirada   string    `json:"responsavel_retirada" validate:"max=200"`
	ParentescoResponsavel string    `json:"parentesco_responsavel" validate:"max=100"`
	MotivoObservacao      string    `json:"motivo_observacao" validate:"max=2000"`
	DataTermo             time.Time `json:"data_termo" validate:"required"`
}

// RegistrarTermo registra um termo de advertência, passeio ou saída.
func (h *Handlers) RegistrarTermo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TermoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.RegistrarTermo(r.Context(), identity(r), id, req.TipoTermo, req.ResponsavelRetirada,
		req.ParentescoResponsavel, req.MotivoObservacao, req.DataTermo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, t)
}

// RemoverTermo remove um termo registrado por engano.
func (h *Handlers) RemoverTermo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverTermo(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// TermosCasaAbrigo lista os termos recentes da unidade.
func (h *Handlers) TermosCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.TermosDaUnidade(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ------------------------------------------------------------------- PIA

// SalvarPiaFicha grava a ficha do PIA (as 6 abas).
func (h *Handlers) SalvarPiaFicha(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req domain.PiaFichaDados
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SalvarPiaFicha(r.Context(), identity(r), id, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// PiaMetaRequest é uma meta do plano de ação ou da intervenção do PIA.
type PiaMetaRequest struct {
	Tipo        string     `json:"tipo" validate:"required,oneof=plano intervencao"`
	Descricao   string     `json:"descricao" validate:"required,min=3,max=1000"`
	Providencia string     `json:"providencia" validate:"max=1000"`
	Responsavel string     `json:"responsavel" validate:"max=200"`
	Prazo       *time.Time `json:"prazo"`
	Esportes    string     `json:"esportes" validate:"max=500"`
	Cultura     string     `json:"cultura" validate:"max=500"`
	Lazer       string     `json:"lazer" validate:"max=500"`
}

func (req PiaMetaRequest) dados() domain.PiaMetaDados {
	return domain.PiaMetaDados{Tipo: req.Tipo, Descricao: req.Descricao, Providencia: req.Providencia, Responsavel: req.Responsavel,
		Prazo: req.Prazo, Esportes: req.Esportes, Cultura: req.Cultura, Lazer: req.Lazer}
}

// AdicionarPiaMeta registra uma meta do PIA.
func (h *Handlers) AdicionarPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PiaMetaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.AdicionarPiaMeta(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

// EditarPiaMeta regrava uma meta do PIA.
func (h *Handlers) EditarPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PiaMetaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.EditarPiaMeta(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

// ConclusaoPiaMetaRequest é a (des)conclusão de uma meta do PIA.
type ConclusaoPiaMetaRequest struct {
	Concluida bool `json:"concluida"`
}

// ConcluirPiaMeta marca (ou desmarca) uma meta do PIA como concluída.
func (h *Handlers) ConcluirPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ConclusaoPiaMetaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.ConcluirPiaMeta(r.Context(), identity(r), id, req.Concluida)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

// RemoverPiaMeta remove uma meta registrada por engano.
func (h *Handlers) RemoverPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverPiaMeta(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// ------------------------------------------------------------ documentos

// UploadDocumentoRequest inicia o upload de um documento do acolhido ou do responsável.
type UploadDocumentoRequest struct {
	Tipo        string `json:"tipo" validate:"required,oneof=acolhido responsavel"`
	Filename    string `json:"filename" validate:"required,max=255"`
	ContentType string `json:"content_type" validate:"required,max=150"`
	Size        int64  `json:"size" validate:"required,min=1"`
}

// StartUploadDocumento cria o metadado pendente e a URL de upload direto ao object storage.
func (h *Handlers) StartUploadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req UploadDocumentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.StartUploadDocumento(r.Context(), identity(r), id, req.Tipo, req.Filename, req.ContentType, req.Size)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, res)
}

// ConfirmUploadDocumento confere o objeto enviado e publica o documento.
func (h *Handlers) ConfirmUploadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.ConfirmUploadDocumento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, d)
}

// DownloadDocumento devolve uma URL pré-assinada de leitura (curta).
func (h *Handlers) DownloadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	url, d, err := h.svc.DownloadURLDocumento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]any{"url": url, "documento": d})
}

// RemoverDocumento remove o documento (metadado + objeto no storage).
func (h *Handlers) RemoverDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverDocumento(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// ------------------------------------------------- painel e relatórios

// PainelCasaAbrigo devolve os cartões, os acolhidos e o monitor de prazos.
func (h *Handlers) PainelCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.PainelCasaAbrigo(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// dataQuery lê uma data AAAA-MM-DD da query string.
func dataQuery(r *http.Request, nome string) (time.Time, error) {
	d, err := time.Parse(time.DateOnly, httputil.Query(r, nome, 10))
	if err != nil {
		return time.Time{}, &domain.ValidationError{Msg: nome + ": data inválida (AAAA-MM-DD)"}
	}
	return d, nil
}

// EstatisticasCasaAbrigo devolve o relatório estatístico de ?de=&ate=.
func (h *Handlers) EstatisticasCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	de, err := dataQuery(r, "de")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ate, err := dataQuery(r, "ate")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.EstatisticasCasaAbrigo(r.Context(), identity(r), id, de, ate)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

// EvolucoesCasaAbrigo lista as evoluções dos acolhidos da unidade.
func (h *Handlers) EvolucoesCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.EvolucoesCasaAbrigo(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ---------------------------------------------------------------- diário

// AnotacaoDiarioRequest é uma anotação do diário de bordo.
type AnotacaoDiarioRequest struct {
	AcolhimentoID  *uuid.UUID `json:"acolhimento_id"`
	DataReferencia time.Time  `json:"data_referencia" validate:"required"`
	Texto          string     `json:"texto" validate:"required,min=3,max=4000"`
}

// RegistrarAnotacaoDiario inclui uma anotação no diário de bordo.
func (h *Handlers) RegistrarAnotacaoDiario(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AnotacaoDiarioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.RegistrarAnotacaoDiario(r.Context(), identity(r), id, req.AcolhimentoID, req.DataReferencia, req.Texto)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, a)
}

// DiarioCasaAbrigo lista as anotações do mês ?mes=AAAA-MM (padrão: o atual).
func (h *Handlers) DiarioCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	mes := httputil.Query(r, "mes", 10)
	if mes == "" {
		mes = time.Now().Format("2006-01")
	}
	out, err := h.svc.DiarioCasaAbrigo(r.Context(), identity(r), id, mes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ------------------------------------------------------------ relatórios

// RelatorioRequest é um relatório escrito.
type RelatorioRequest struct {
	AcolhimentoID *uuid.UUID `json:"acolhimento_id"`
	Tipo          string     `json:"tipo" validate:"required,max=40"`
	TipoOutro     string     `json:"tipo_outro" validate:"max=200"`
	Status        string     `json:"status" validate:"required,max=40"`
	DataEntrega   time.Time  `json:"data_entrega" validate:"required"`
	Conteudo      string     `json:"conteudo" validate:"max=100000"`
}

func (req RelatorioRequest) dados() domain.RelatorioDados {
	return domain.RelatorioDados{AcolhimentoID: req.AcolhimentoID, Tipo: req.Tipo, TipoOutro: req.TipoOutro, Status: req.Status,
		DataEntrega: req.DataEntrega, Conteudo: req.Conteudo}
}

// RegistrarRelatorio cria um relatório escrito.
func (h *Handlers) RegistrarRelatorio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req RelatorioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	rel, err := h.svc.RegistrarRelatorio(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, rel)
}

// AtualizarRelatorio regrava um relatório (o entregue não muda mais).
func (h *Handlers) AtualizarRelatorio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req RelatorioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	rel, err := h.svc.AtualizarRelatorio(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, rel)
}

// RemoverRelatorio exclui um relatório ainda não entregue.
func (h *Handlers) RemoverRelatorio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverRelatorio(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// RelatoriosCasaAbrigo lista os relatórios escritos da unidade.
func (h *Handlers) RelatoriosCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.RelatoriosCasaAbrigo(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// --------------------------------------------------------------- tarefas

// TarefaRequest é uma tarefa do Kanban da equipe.
type TarefaRequest struct {
	Titulo      string     `json:"titulo" validate:"required,min=2,max=200"`
	Descricao   string     `json:"descricao" validate:"max=4000"`
	Status      string     `json:"status" validate:"omitempty,oneof=backlog todo doing review done"`
	Prioridade  string     `json:"prioridade" validate:"omitempty,oneof=baixa media alta critica"`
	Responsavel string     `json:"responsavel" validate:"max=200"`
	Sprint      string     `json:"sprint" validate:"max=100"`
	Prazo       *time.Time `json:"prazo"`
	Ordem       int        `json:"ordem" validate:"min=0"`
}

func (req TarefaRequest) dados() domain.TarefaDados {
	return domain.TarefaDados{Titulo: req.Titulo, Descricao: req.Descricao, Status: req.Status, Prioridade: req.Prioridade,
		Responsavel: req.Responsavel, Sprint: req.Sprint, Prazo: req.Prazo, Ordem: req.Ordem}
}

// CriarTarefa cria uma tarefa no Kanban da equipe.
func (h *Handlers) CriarTarefa(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TarefaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.CriarTarefa(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, t)
}

// AtualizarTarefa regrava a tarefa (inclusive a coluna, ao arrastar).
func (h *Handlers) AtualizarTarefa(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TarefaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.AtualizarTarefa(r.Context(), identity(r), id, req.dados())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, t)
}

// RemoverTarefa exclui uma tarefa.
func (h *Handlers) RemoverTarefa(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverTarefa(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// TarefasCasaAbrigo lista o quadro da equipe.
func (h *Handlers) TarefasCasaAbrigo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.TarefasCasaAbrigo(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}
