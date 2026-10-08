// Package migrations embute os scripts SQL (goose) no binário de
// migração — a imagem de produção não depende de arquivos soltos.
package migrations

import "embed"

// FS contém todos os arquivos .sql deste diretório.
//
//go:embed *.sql
var FS embed.FS
