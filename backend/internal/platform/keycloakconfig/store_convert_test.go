package keycloakconfig

import (
	"testing"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

func TestSettings_ToKeycloakConfig(t *testing.T) {
	s := Settings{
		IssuerURL:        "https://sso.orgao.gov.br/realms/nexus",
		Realm:            "nexus",
		ClientID:         "nexus-backend",
		ClientSecret:     "segredo",
		Audience:         "nexus-backend",
		FrontendClientID: "nexus-frontend", // não faz parte de config.KeycloakConfig — deve ser ignorado
	}

	want := config.KeycloakConfig{
		IssuerURL:    "https://sso.orgao.gov.br/realms/nexus",
		Realm:        "nexus",
		ClientID:     "nexus-backend",
		ClientSecret: "segredo",
		Audience:     "nexus-backend",
	}

	if got := s.ToKeycloakConfig(); got != want {
		t.Errorf("ToKeycloakConfig() = %+v, want %+v", got, want)
	}
}
