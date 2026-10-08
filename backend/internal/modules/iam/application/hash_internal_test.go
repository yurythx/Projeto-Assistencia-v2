package application

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
)

// Falha ao gerar o hash (sem entropia, por exemplo) é erro interno, e
// nenhuma conta é criada nem senha trocada.
func TestPasswordHashFailureIsInternal(t *testing.T) {
	s := &Service{hash: func(string) (string, error) { return "", errors.New("sem entropia") }}
	_, err := s.CreateLocalUser(context.Background(), CreateLocalUserInput{Username: "valido", Password: "Senha-Forte-123!"})
	if appErr, ok := apperrors.As(err); !ok || appErr.Status != http.StatusInternalServerError {
		t.Errorf("criar conta com falha no hash: %v", err)
	}
	err = s.ResetPassword(context.Background(), uuid.New(), "Senha-Forte-123!")
	if appErr, ok := apperrors.As(err); !ok || appErr.Status != http.StatusInternalServerError {
		t.Errorf("redefinir senha com falha no hash: %v", err)
	}
}
