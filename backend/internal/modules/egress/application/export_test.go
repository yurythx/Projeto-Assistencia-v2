package application

import "context"

// DeliverBatch expõe aos testes uma rodada completa do worker de entregas,
// síncrona: esperar um tempo fixo pelo RunDeliveries em segundo plano
// dependia da velocidade da máquina (no CI com -race a rodada às vezes não
// terminava a tempo e a entrega ficava pendente).
func (s *Service) DeliverBatch(ctx context.Context) { s.deliverBatch(ctx) }
