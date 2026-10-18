package application

import (
	"context"
	"time"
)

// AvisarACada expõe o laço do worker com um intervalo curto (testes).
func (s *Service) AvisarACada(ctx context.Context, intervalo time.Duration) error {
	return s.avisarACada(ctx, intervalo)
}
