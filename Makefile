.PHONY: dev up down logs build test lint format \
	deploy prod-seed-admin prod-ps prod-logs prod-limpeza prod-down iam-scope-report ia-modelo estrutura-gerar estrutura-aplicar ttdd-impacto ttdd-aplicar \
	demo-keycloak demo-generate demo-seed demo-test demo-popular \
	migrate-up migrate-down migrate-status migrate-redo seed-admin \
	backend-shell frontend-shell rabbitmq-status clean \
	backend-build backend-test backend-lint backend-sec backend-format \
	frontend-build frontend-test frontend-lint frontend-format

ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE      := docker compose -f docker-compose.yml -f docker-compose.dev.yml
# Sem -f: respeita COMPOSE_FILE do .env (Keycloak de teste); padrão = docker-compose.yml.
COMPOSE_PROD := docker compose
GOOSE_DIR    := backend/migrations
DB_USER      ?= nexus
DB_PASSWORD  ?= nexus_pass
DB_NAME      ?= nexus
HOST_DB_PORT ?= 5433
DB_DSN       ?= postgres://$(DB_USER):$(DB_PASSWORD)@localhost:$(HOST_DB_PORT)/$(DB_NAME)?sslmode=disable

## --- Orquestração local ---

dev: ## Sobe todos os serviços em modo desenvolvimento (com overrides de dev)
	$(COMPOSE) up --build

up: ## Sobe todos os serviços em modo produção
	$(COMPOSE_PROD) up --build -d

## --- Servidor (produção, só docker-compose.yml — ver docs/DEPLOY.md) ---

deploy: ## git pull + gera .env/chave na 1ª vez + build + sobe e espera healthy
	./scripts/deploy.sh

prod-seed-admin: ## Cria/reseta o admin local dentro do container (sem Go no host)
	$(COMPOSE_PROD) run --rm --no-deps --entrypoint ./seedadmin backend-api

prod-ps: ## Estado dos serviços de produção
	$(COMPOSE_PROD) ps

prod-limpeza: ## Libera disco: cache de build sem uso há 3 dias e imagens órfãs (nunca volumes)
	docker builder prune -a -f --filter until=72h
	docker image prune -f
	df -h /

prod-logs: ## Logs dos serviços de produção
	$(COMPOSE_PROD) logs -f --tail=200

prod-down: ## Para os serviços de produção (mantém os volumes)
	$(COMPOSE_PROD) down

iam-scope-report: ## Impacto da permissão com escopo (ADR 013) nas lotações e grupos atuais
	$(COMPOSE_PROD) exec -T postgres psql -U $(DB_USER) -d $(DB_NAME) < scripts/iam-scope-report.sql

estrutura-gerar: ## Regera deploy/estrutura a partir do export do AD (AD=docs/AD)
	node scripts/estrutura/ad-para-estrutura.mjs $${AD:-docs/AD}

ttdd-impacto: ## Mostra o que a carga da TTDD mudaria (séries novas, alteradas, revogadas e procedimentos afetados), sem gravar
	cat deploy/ttdd/ttdd.sql deploy/ttdd/carga.sql | $(COMPOSE_PROD) exec -T postgres psql -q -v ON_ERROR_STOP=1 -v aplicar=0 -U $(DB_USER) -d $(DB_NAME)

ttdd-aplicar: ## Carrega a TTDD oficial (deploy/ttdd, gerada de docs/ttdd.pdf): mostra o impacto e grava, com histórico e revogação
	cat deploy/ttdd/ttdd.sql deploy/ttdd/carga.sql | $(COMPOSE_PROD) exec -T postgres psql -q -v ON_ERROR_STOP=1 -v aplicar=1 -U $(DB_USER) -d $(DB_NAME)

estrutura-aplicar: ## Aplica (idempotente) a estrutura organizacional real no Postgres
	$(COMPOSE_PROD) exec -T postgres psql -q -v ON_ERROR_STOP=1 -U $(DB_USER) -d $(DB_NAME) < deploy/estrutura/estrutura.sql

## --- Ambiente de teste: Keycloak + dados fictícios (ver docs/DEPLOY.md) ---

demo-keycloak: ## Liga o Keycloak de teste, sobe a stack e aplica os dados fictícios
	./scripts/demo-keycloak.sh

demo-generate: ## Regera realm do Keycloak + SQL + CSV a partir de scripts/demo-data
	node scripts/demo-data/generate.mjs

demo-seed: ## Aplica (idempotente) a estrutura organizacional e os usuários fictícios no Postgres
	$(COMPOSE_PROD) exec -T postgres psql -q -v ON_ERROR_STOP=1 -U $(DB_USER) -d $(DB_NAME) < deploy/demo/seed-demo.sql

demo-test: ## Cenários ponta a ponta com os usuários fictícios (login real pelo Keycloak)
	python3 scripts/demo-data/cenarios/tramite.py
	python3 scripts/demo-data/cenarios/modulos.py
	python3 scripts/demo-data/cenarios/conteudo.py
	python3 scripts/demo-data/cenarios/colaboracao.py
	python3 scripts/demo-data/cenarios/tramite_documentos.py
	python3 scripts/demo-data/cenarios/integracoes.py
	python3 scripts/demo-data/cenarios/plataforma.py

demo-popular: ## Povoa todos os apps com dados fictícios realistas (uma vez)
	cd scripts/demo-data/cenarios && python3 popular.py

down: ## Para e remove todos os serviços
	$(COMPOSE) down

logs: ## Acompanha os logs de todos os serviços
	$(COMPOSE) logs -f

ia-modelo: ## Baixa (ou atualiza) o modelo IA_LOCAL_MODELO no serviço ia-local (overlay docker-compose.ia.yml)
	$(COMPOSE_PROD) run --rm ia-local-modelo

clean: ## Para os serviços e remove os volumes (DESTRÓI os dados locais)
	$(COMPOSE) down -v

## --- Build ---

build: backend-build frontend-build ## Compila os binários/artefatos do backend e do frontend

backend-build:
	cd backend && go build ./...

frontend-build:
	cd frontend && npm run build

## --- Testes ---

test: backend-test frontend-test ## Roda as suítes de teste do backend e do frontend

backend-test: 
	cd backend && go test ./... -p 1

frontend-test:
	cd frontend && npm test

## --- Lint / format ---

lint: backend-lint frontend-lint ## Roda lint no backend e no frontend

backend-lint:
	cd backend && go vet ./...

backend-sec: ## SAST do backend (govulncheck + staticcheck + gosec) — mesmo conjunto do CI
	cd backend && govulncheck ./...
	cd backend && staticcheck ./...
	cd backend && gosec -quiet -exclude=G104 ./...

frontend-lint:
	cd frontend && npm run lint

format: backend-format frontend-format ## Formata o backend e o frontend

backend-format:
	cd backend && go fmt ./...

frontend-format:
	cd frontend && npm run format

## --- Migrations (Goose) ---

migrate-up: ## Aplica todas as migrations pendentes
	cd $(GOOSE_DIR) && goose postgres "$(DB_DSN)" up

migrate-down: ## Reverte a última migration
	cd $(GOOSE_DIR) && goose postgres "$(DB_DSN)" down

migrate-status: ## Mostra o status das migrations
	cd $(GOOSE_DIR) && goose postgres "$(DB_DSN)" status

migrate-redo: ## Exercita a reversibilidade: up -> down-to 0 -> up (toda migration precisa de Down válido)
	cd $(GOOSE_DIR) && goose postgres "$(DB_DSN)" up \
		&& goose postgres "$(DB_DSN)" down-to 0 \
		&& goose postgres "$(DB_DSN)" up \
		&& goose postgres "$(DB_DSN)" status

seed-admin: ## Cria/reseta o usuário admin local com senha ALEATÓRIA
	cd backend && DB_HOST=localhost DB_PORT=$(HOST_DB_PORT) DB_NAME=$(DB_NAME) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) \
		go run ./cmd/seedadmin

new-module: ## Gera o esqueleto Clean Architecture de um novo módulo (Uso: make new-module NAME=contratos)
	@./scripts/create-module.sh $(NAME)

## --- Shells ---

backend-shell: ## Abre um shell no container backend-api em execução
	$(COMPOSE) exec backend-api sh

frontend-shell: ## Abre um shell no container frontend em execução
	$(COMPOSE) exec frontend sh

## --- RabbitMQ ---

rabbitmq-status: ## Mostra o status do nó/filas do RabbitMQ
	$(COMPOSE) exec rabbitmq rabbitmqctl status
