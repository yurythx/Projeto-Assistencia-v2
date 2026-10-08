package auth

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// O relatório de impacto (scripts/iam-scope-report.sql) repete a lista de
// permissões com escopo: as duas precisam andar juntas.
func TestRelatorioDeImpactoEspelhaScopedPermissions(t *testing.T) {
	sql, err := os.ReadFile("../../../../scripts/iam-scope-report.sql")
	if err != nil {
		t.Fatal(err)
	}
	var noSQL []string
	for _, m := range regexp.MustCompile(`'([a-z]+:[a-z]+)'`).FindAllStringSubmatch(string(sql), -1) {
		noSQL = append(noSQL, m[1])
	}
	var noGo []string
	for _, p := range ScopedPermissions {
		noGo = append(noGo, string(p))
	}
	slices.Sort(noSQL)
	slices.Sort(noGo)
	if !slices.Equal(noSQL, noGo) {
		t.Fatalf("iam-scope-report.sql %v difere de ScopedPermissions %v", noSQL, noGo)
	}
}
