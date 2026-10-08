package iaconfig

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Pedido a uma função de IA. Montar recebe a pergunta (já mascarada quando
// a conexão é externa e a função pede) e devolve a mensagem do usuário.
type Pedido struct {
	Sistema     string
	Pergunta    string
	Montar      func(pergunta string) string
	Temperatura float64
	MaxTokens   int
}

// Resposta da IA e a conexão que a produziu.
type Resposta struct {
	Texto   string
	Conexao string
}

// Roteador escolhe a conexão de cada chamada: principal, depois reserva.
// Lê a configuração a cada chamada (sem cache): a troca na tela vale na
// próxima pergunta, em todas as instâncias da API.
type Roteador struct {
	store    Store
	cliente  *Cliente
	ambiente *Conexao // ATLAS_AI_* (nil = sem IA quando nada foi configurado)
	logger   *slog.Logger
}

// NovoRoteador cria o roteador. ambiente é a conexão do .env, usada só
// enquanto a função não foi configurada pela tela.
func NovoRoteador(store Store, cliente *Cliente, ambiente *Conexao, logger *slog.Logger) *Roteador {
	return &Roteador{store: store, cliente: cliente, ambiente: ambiente, logger: logger}
}

// ConexaoDoAmbiente monta a conexão das variáveis de ambiente (nil se o
// endereço está vazio). Nunca é tratada como externa: quem a definiu no
// servidor já decidiu para onde os dados vão.
func ConexaoDoAmbiente(endpoint, chave, modelo string, timeoutSegundos int) *Conexao {
	if strings.TrimSpace(endpoint) == "" {
		return nil
	}
	return &Conexao{Nome: "Variáveis de ambiente (ATLAS_AI_*)", Provedor: ProvedorCompativel, Endpoint: endpoint,
		Chave: chave, Modelo: modelo, TimeoutSegundos: max(timeoutSegundos, 1)}
}

// Ambiente devolve a conexão do .env (nil se não houver).
func (r *Roteador) Ambiente() *Conexao { return r.ambiente }

// candidatas devolve as conexões da função, em ordem, e se a pergunta
// deve ser mascarada para as externas.
func (r *Roteador) candidatas(ctx context.Context, funcao string) ([]Conexao, bool, error) {
	u, err := r.store.Uso(ctx, funcao)
	if err != nil {
		return nil, false, err
	}
	if !u.Configurado {
		if r.ambiente == nil {
			return nil, false, ErrSemConexao
		}
		return []Conexao{*r.ambiente}, false, nil
	}
	var out []Conexao
	for _, id := range []*uuid.UUID{u.PrincipalID, u.ReservaID} {
		if id == nil {
			continue
		}
		c, err := r.store.Obter(ctx, *id)
		if err != nil {
			return nil, false, err
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, false, ErrSemConexao
	}
	return out, u.MascararDadosPessoais, nil
}

// Conversar atende o pedido com a primeira conexão que responder. Sem
// conexão: ErrSemConexao (a função segue sem IA). Todas falhando: o erro
// da última.
func (r *Roteador) Conversar(ctx context.Context, funcao string, p Pedido) (Resposta, error) {
	conexoes, mascarar, err := r.candidatas(ctx, funcao)
	if err != nil {
		return Resposta{}, err
	}
	var ultimo error
	for _, c := range conexoes {
		pergunta := p.Pergunta
		if c.Externo && mascarar {
			pergunta = MascararDadosPessoais(pergunta)
		}
		texto, err := r.cliente.Conversar(ctx, c, []Mensagem{
			{Papel: "system", Conteudo: p.Sistema},
			{Papel: "user", Conteudo: p.Montar(pergunta)},
		}, p.Temperatura, p.MaxTokens)
		if err == nil {
			return Resposta{Texto: texto, Conexao: c.Nome}, nil
		}
		r.logger.WarnContext(ctx, "ia: conexão falhou, tentando a próxima", "funcao", funcao, "conexao", c.Nome, "error", err)
		ultimo = err
	}
	return Resposta{}, ultimo
}
