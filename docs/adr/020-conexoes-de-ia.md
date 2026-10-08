# 020 — Conexões de IA configuráveis pela tela; IA local como serviço próprio

- **Status:** Aceito e implementado
- **Data:** 2026-10-03
- **Relacionado:** [ADR 015](015-atlas-padronizado.md) (Atlas: grounding e IA opcional), keycloakconfig (configuração em runtime com segredo cifrado)

## Contexto

O assistente do Atlas redige respostas com um modelo de linguagem, sempre
preso ao contexto homologado (ADR 015). A IA era definida só por
variáveis de ambiente (`ATLAS_AI_*`): trocar de fornecedor, de modelo ou de
chave exigia acesso ao servidor e novo deploy; não havia teste da conexão,
reserva, controle sobre envio de dados a fornecedor externo nem visão de
qual IA estava em uso. O Ollama existia como overlay `docker-compose.atlas.yml`
(serviço `ollama`, versão 0.6.8) — e, no servidor de teste, não estava ligado.

Referências de mercado: gateway de IA (LiteLLM, Kong AI Gateway, Azure API
Management, Cloudflare AI Gateway) — chaves centralizadas e cifradas,
fornecedor de reserva, API compatível com a da OpenAI como interface comum;
política de dados com modelo local por padrão e consentimento explícito para
fornecedor externo.

## Decisão

1. **Conexões de IA** (`internal/platform/iaconfig`, migration `000136`):
   fornecedor (catálogo: IA local/Ollama, OpenAI, Azure OpenAI, Gemini,
   Anthropic, Groq, OpenRouter, Mistral, "compatível"), endereço, modelo,
   tempo limite e chave. A chave é cifrada com `CONFIG_ENCRYPTION_KEY`
   (AES-256-GCM, como os segredos do Keycloak), nunca volta pela API (só os
   4 últimos caracteres) e nunca vai para a auditoria nem para o log.
2. **Testar antes de salvar:** criar ou alterar faz uma conversa real; se o
   fornecedor não responder, nada é gravado e o motivo vem traduzido
   (chave recusada, modelo não encontrado, limite de uso, sem resposta no
   tempo limite, DNS, certificado). O resultado do último teste fica salvo.
3. **A chave não muda de endereço:** alterar o endereço exige informar a
   chave de novo — senão, quem tem `ia:manage` poderia mandar para um
   servidor próprio uma chave que a tela não mostra. Endereços link-local
   (metadados de nuvem) são recusados.
4. **Uso por função** (`ia_uso`): o assistente do Atlas tem **principal** e
   **reserva**; falhando as duas, a síntese canônica. Sem linha salva valem
   as variáveis de ambiente; principal nula = IA desligada. O roteador lê a
   configuração a cada chamada: a troca vale na próxima pergunta, em todas
   as instâncias, sem reiniciar.
5. **Fornecedor externo (LGPD):** associar uma conexão externa exige
   autorização explícita, registrada (quem e quando); por padrão, CPF, CNPJ,
   e-mail e telefone são mascarados na pergunta antes do envio. O contexto
   (procedimentos e TTDD) é público e segue sem máscara.
6. **Permissão própria** `ia:manage` (Configurações → Inteligência
   artificial), auditoria de toda alteração (`ia.conexao.*`, `ia.uso.alterado`).
7. **IA local como serviço próprio:** overlay `docker-compose.ia.yml`,
   serviço `ia-local` (Ollama 0.35.1, fixado), sem porta publicada, com
   limite de memória e CPU, um modelo e uma geração por vez; o serviço
   `ia-local-modelo` baixa o modelo (`IA_LOCAL_MODELO`, padrão
   `qwen2.5:1.5b`) na subida. `make ia-modelo` baixa de novo.

## Consequências

- **Medição no servidor de teste** (Ryzen 5 PRO 4650G, 4 vCPU, **4 GB**,
  sem GPU): o `qwen2.5:1.5b` respondeu corretamente ("100 anos, depois
  eliminação"), leu o contexto a ~115 tokens/s, mas **gerou a 0,3
  token/s**: com a stack (~2 GB) e o modelo (~1,2 GB), a memória acabou, o
  sistema passou a reler o disco sem parar e a máquina inteira ficou
  lenta. IA local exige ≥ 8 GB de RAM, uma máquina dedicada (a conexão
  "IA local" aceita qualquer endereço da rede) ou GPU. O overlay **não**
  foi ligado no servidor de teste.
- Fornecedor externo responde em segundos, mas os dados saem da rede:
  contrato sem retenção de dados e a autorização registrada na tela.
- Fica para depois: painel de uso (latência, falhas, tokens, custo),
  resposta em streaming e um conjunto de perguntas de referência para
  comparar modelos antes de trocar.
