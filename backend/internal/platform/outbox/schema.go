package outbox

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
)

// schemaJSON embute o JSON Schema do envelope de evento no próprio binário
// — não há leitura de disco em produção, e o schema viaja versionado
// junto do código que o usa, sem risco de arquivo ausente/desatualizado
// num deploy.
//
//go:embed envelope.schema.json
var schemaJSON []byte

const schemaResourceID = "https://projeto-nexus.internal/schemas/event-envelope.json"

var (
	compileOnce    sync.Once
	compiledSchema *jsonschema.Schema
	compileErr     error
)

// envelopeSchema compila o JSON Schema embutido exatamente uma vez (é
// imutável em tempo de execução — vem do binário, não de configuração) e
// reutiliza o *jsonschema.Schema resultante em toda chamada a Validate.
// É uma variável para os testes simularem um schema que não compila.
var envelopeSchema = func() (*jsonschema.Schema, error) {
	compileOnce.Do(func() { compiledSchema, compileErr = compileSchema(schemaJSON) })
	return compiledSchema, compileErr
}

func compileSchema(raw []byte) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaResourceID, bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("outbox: load embedded schema: %w", err)
	}
	schema, err := compiler.Compile(schemaResourceID)
	if err != nil {
		return nil, fmt.Errorf("outbox: compile embedded schema: %w", err)
	}
	return schema, nil
}

// validateEnvelope confere envelope (o JSON já serializado de um
// events.Event) contra o schema padrão do envelope (§17): id/correlation_id
// como UUID, type seguindo a convenção "<contexto>.<entidade>.<ação>"
// (§9), version/source/occurred_at/payload presentes com o tipo certo, e
// nenhum campo extra fora do contrato. É a última linha de defesa antes
// do INSERT em outbox_events — mesmo que o envelope viesse de outro
// caminho que não events.New (um bug futuro, um refactor descuidado), um
// evento fora do formato padrão nunca chega a ser persistido nem, por
// consequência, publicado no RabbitMQ (o Publisher só publica o que está
// em outbox_events).
func validateEnvelope(envelope []byte) error {
	schema, err := envelopeSchema()
	if err != nil {
		// Um schema que não compila é um bug de build/deploy, não uma
		// falha de dados — mas nunca deve resultar em eventos passando
		// sem validação nenhuma.
		return apperrors.Internal(fmt.Errorf("outbox: schema unavailable: %w", err))
	}

	var doc any
	if err := json.Unmarshal(envelope, &doc); err != nil {
		return apperrors.Internal(fmt.Errorf("outbox: envelope is not valid JSON: %w", err))
	}

	if err := schema.Validate(doc); err != nil {
		return apperrors.Internal(fmt.Errorf("outbox: envelope does not match the standard event contract: %w", err))
	}
	return nil
}
