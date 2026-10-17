# Homologação da SEMPRAS

Roteiro para a TI montar um ambiente onde a equipe de cada tipo de unidade
testa a própria rotina, com login real e dados fictícios, **sem tocar na
produção**.

## Onde rodar

Em qualquer máquina com Docker (16 GB de RAM recomendados), **não no
Proxmox**. Em 2026-10-17 o host `pve1` tinha 11 GB, com 2,7 GB livres e
três VMs de produção que somam 12 GB alocados. Uma VM a mais poria em risco
a produção deste e de outros sistemas. Se o host ganhar memória, a mesma
receita roda numa VM dedicada.

## Montar

Na raiz do repositório:

```bash
cp .env.example .env                       # ajuste as senhas, se quiser
mkdir -p secrets && openssl genrsa -out secrets/local_auth_private_key.pem 2048
C="docker compose -p homolog -f docker-compose.yml -f docker-compose.dev.yml"
$C up -d --build
$C exec -T postgres psql -U nexus -d nexus < deploy/estrutura/estrutura.sql
$C exec -T postgres psql -U nexus -d nexus < deploy/sempras/tipos-unidades.sql
$C run --rm --no-deps --entrypoint ./seedadmin backend-api   # anote a senha do admin
```

Entre como `admin` em http://localhost:3002 e ligue o módulo em
**Configurações → Módulos → SEMPRAS**.

## Usuários e dados fictícios

```bash
python3 scripts/homologacao/sempras.py --api http://localhost:8002/api/v1 \
  --admin admin --senha-admin '<senha do admin>' --senha '<senha dos testadores>'
```

O script cria um usuário por perfil para uma unidade de cada tipo, mais a
superintendência:

- `homolog.cras.recepcao`, `homolog.cras.tecnico`, `homolog.cras.psicologo`
- o mesmo para `scfv`, `creas`, `centro-pop`, `casa-mulher`, `casa-abrigo`
  e `conselho-tutelar`
- `homolog.superintendencia`

Ele também cria munícipes e atendimentos fictícios em cada unidade: fila,
PAIF, PAEFI, abordagem, caso da Casa da Mulher, acolhimento e Conselho.

O script se recusa a rodar contra o host da produção.

## O que cada testador confere

Cada um entra com o próprio usuário e segue o [guia de uso](GUIA_SEMPRAS.md):

- [ ] Vejo só a minha unidade, com os serviços do meu tipo.
- [ ] **Recepção:** busco, cadastro, escolho a demanda, incluo na fila e
  imprimo a senha.
- [ ] **Técnico:** chamo, atendo, registro a evolução e uso os serviços
  próprios da unidade.
- [ ] **Psicologia:** a evolução psicológica só aparece para mim.
- [ ] **Painel da TV:** com o nome no CRAS, SCFV e Centro POP; só a senha
  nas unidades sigilosas.
- [ ] **Encaminhamento:** um CRAS encaminha para o CREAS; o CREAS aceita e
  vê o resumo.
- [ ] **Superintendência:** painel, RMA por formulário e tela de Unidades.
- [ ] Anote o que falta na rotina da unidade (campo, lista, relatório).

## Desmontar

```bash
docker compose -p homolog -f docker-compose.yml -f docker-compose.dev.yml down -v
```
