# RIPD — Relatório de Impacto à Proteção de Dados Pessoais

> **Modelo / minuta técnica.** Este documento consolida o que a engenharia
> sabe sobre os fluxos de dados pessoais do Projeto Nexus. O RIPD
> **oficial** é elaborado e assinado pelo Encarregado de Dados (DPO) do
> órgão, que valida bases legais, necessidade e proporcionalidade. As
> seções abaixo são o insumo técnico (art. 5º, XVII e art. 38 da LGPD).

- **Sistema:** Projeto Nexus — base tecnológica dos sistemas municipais
- **Controlador:** Prefeitura Municipal de Rondonópolis/MT
- **Operadores:** provedores de hospedagem contratados; Login Único Gov.br / Keycloak (autenticação)
- **Versão dos termos vigente:** `v1.0.0-2026`
- **Data desta minuta:** 2026-09-09

---

## 1. Descrição do tratamento

| Aspecto | Descrição |
|---|---|
| Natureza | Autenticação, autorização, auditoria e prestação de serviços públicos digitais |
| Escopo | Servidores públicos e, nos sistemas derivados, cidadãos |
| Contexto | Ambiente interno de governo; acesso via Login Único Gov.br ou login local |
| Finalidades | Ver Política de Privacidade `/privacidade`, seção 3 |

## 2. Dados pessoais tratados e base legal

| Categoria | Dados | Base legal (LGPD) | Retenção |
|---|---|---|---|
| Identificação | usuário, e-mail, nome, hash de senha, sub Gov.br/Keycloak, papéis | art. 7º III (política pública); art. 7º I para login local | enquanto a conta estiver ativa |
| Registros de acesso | data/hora login/logout, IP, correlation_id, ações (trilha) | art. 7º II (obrigação legal — Marco Civil, controle interno) | prazo da legislação de controle (a definir pelo DPO) |
| Consentimento | versão, data/hora, IP, user-agent | art. 7º II + art. 8º §1º | permanente (prova) |
| Preferências de UI | contraste, fonte, tema | art. 7º IX (interesse legítimo) | só no navegador do titular |
| Consentimento anônimo | device_hash opaco, versão, IP, user-agent | art. 7º II | permanente (prova) |
| Pergunta ao assistente do Atlas (fluxos e TTDD) | texto livre digitado pelo servidor — pode conter dados pessoais de terceiros | art. 7º III (execução de política pública — gestão documental) | **não é armazenada**: a auditoria guarda só modo, relevância, séries usadas e tamanho (ADR 015/021) |
| Notificações e seguimento de fluxos | destinatário, título/mensagem do aviso, link interno, lido em; procedimentos seguidos; preferência por módulo (ADR 027) | art. 7º III (execução de política pública) e art. 7º IX (interesse legítimo na comunicação de mudanças) | aviso lido: até 180 dias; apagados com a conta; o titular desliga os avisos por módulo |
| Entrevistas de validação de fluxos | departamento, participantes (registrados pela função, não pelo nome), registro e pendências (ADR 028) | art. 7º III (execução de política pública — gestão documental) | enquanto o procedimento existir (histórico da validação); a auditoria não guarda os participantes |

**Dados sensíveis (art. 11):** nenhum tratado pela base. Sistemas
derivados que tratem dados sensíveis fazem seu próprio RIPD.

## 3. Fluxo de dados

```
Cidadão/Servidor
  → Gov.br / Keycloak (OIDC)  ──token──►  Frontend (cookie de sessão HttpOnly)
  → Frontend  ──BFF (bearer server-side)──►  API Go
       ├─ users (Postgres)                — identificação
       ├─ audit_logs (Postgres, append-only + cópia WORM no object storage)
       ├─ user_consents / anonymous_consents (Postgres)
       └─ data_subject_requests (Postgres)  — pedidos art. 18
  Worker: anonimização (erasure) · export WORM diário com cadeia de hash

  Assistente do Atlas (fluxos e TTDD, ADRs 020/021/023):
  Servidor ──pergunta──► API Go ──fluxos e séries públicos + pergunta──► IA configurada
       IA local (ia-local, na rede)  ou  fornecedor externo (autorização
       registrada; CPF, CNPJ, e-mail e telefone mascarados antes do envio)
```

## 4. Necessidade e proporcionalidade

| Medida de minimização | Situação |
|---|---|
| Listagem de usuários sem e-mail (só id/username/nome) | ✅ implementado (G-10) |
| Mascaramento de PII em log (`slog.LogValuer`) | ✅ disponível |
| IP da prova de consentimento só de proxy confiável | ✅ `TRUSTED_PROXIES` (G-04) |
| Segredos cifrados no banco (AES-256-GCM) | ✅ |
| Acesso à trilha de auditoria restrito a `audit:read` + auto-auditado | ✅ (G-09) |
| Pergunta ao assistente fora da auditoria (só metadados) | ✅ (ADR 015) |
| Mascaramento de CPF/CNPJ/e-mail/telefone antes de fornecedor de IA externo | ✅ padrão ligado (ADR 020) |
| Chave de API de IA cifrada, nunca devolvida nem auditada | ✅ (ADR 020) |

## 5. Riscos e salvaguardas

| Risco | Probabilidade | Impacto | Salvaguarda |
|---|---|---|---|
| Vazamento de credenciais | baixa | alto | bcrypt, cookie HttpOnly, sem token no browser, HSTS/CSP |
| Adulteração da trilha de auditoria | baixa | alto | triggers append-only + cópia WORM externa encadeada por hash (F2.6) |
| Acesso indevido a dados de terceiros | baixa | médio | RBAC por permissão em cada rota + teste negativo no CI |
| DoS / abuso | média | médio | rate limiting distribuído em todo `/api/v1`, timeouts de servidor |
| Retenção excessiva | média | médio | **pendente:** política de retenção por categoria a ser definida pelo DPO |
| XFF forjado poluindo prova de consentimento | baixa | baixo | `httpserver.ClientIP` só confia em `TRUSTED_PROXIES` |
| Pergunta com dados pessoais enviada a fornecedor de IA externo (transferência, inclusive internacional) | média | médio | IA local por padrão; externo só com autorização registrada (quem/quando); mascaramento automático; assistente restrito ao acervo do Atlas — fluxos e TTDD (recusa outros assuntos); contrato sem retenção a cargo do órgão |

## 6. Direitos do titular — operacionalização

| Direito (art. 18) | Canal |
|---|---|
| Acesso / portabilidade | `GET /api/v1/lgpd/meus-dados` (pacote JSON) |
| Eliminação | `POST /api/v1/lgpd/solicitar-exclusao` → anonimização automática (worker) |
| Acompanhamento | `GET /api/v1/lgpd/minhas-solicitacoes` |
| Informação sobre compartilhamento | Política de Privacidade, seção 4 |

## 7. Pendências para o RIPD oficial

- [ ] Definição formal dos **prazos de retenção** por categoria de dado.
- [ ] Designação e publicação do **Encarregado (DPO)** e seu canal.
- [ ] Homologação jurídica das **bases legais** aqui presumidas.
- [ ] Contratos de **operador** com os provedores de hospedagem.
- [ ] Avaliação de **transferência internacional** (se a hospedagem for fora do país).
- [ ] Contrato com o **fornecedor de IA** externo, se usado no assistente do Atlas (operador, sem retenção/treino com os dados, local de processamento) — ADR 020.
