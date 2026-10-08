package audit

import (
	"context"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
)

// FromRequest devolve uma Entry já preenchida com a proveniência da
// requisição HTTP: ator (id interno, subject, roles), IP de origem, user
// agent, contexto organizacional (lotações efetivas) e correlation id. O
// chamador só completa Action/Resource/Before/After/Metadata.
//
// O IP vem de r.RemoteAddr, que o middleware httpserver.TrustedRealIP já
// reescreveu com o IP real do cliente quando a conexão chega por um proxy
// confiável.
func FromRequest(r *http.Request) Entry {
	e := FromContext(r.Context())
	e.IPAddress = r.RemoteAddr
	e.UserAgent = r.UserAgent()
	return e
}

type originKey struct{}

type origin struct{ ip, userAgent string }

// WithOrigin guarda no context o IP de origem e o user agent da requisição
// — a proveniência que a skill exige em TODA entrada de auditoria, mesmo
// nas gravadas pela camada de aplicação, que só recebe o ctx.
func WithOrigin(ctx context.Context, ip, userAgent string) context.Context {
	return context.WithValue(ctx, originKey{}, origin{ip: ip, userAgent: userAgent})
}

// CaptureOrigin é o middleware HTTP que chama WithOrigin. Deve rodar
// depois de httpserver.TrustedRealIP (RemoteAddr já é o IP real).
func CaptureOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
		next.ServeHTTP(w, r.WithContext(WithOrigin(r.Context(), ip, r.UserAgent())))
	})
}

// FromContext preenche ator, contexto organizacional, correlation id e a
// origem (IP/user agent, quando CaptureOrigin rodou) a partir do context —
// para casos de uso da camada de aplicação que recebem só o ctx.
func FromContext(ctx context.Context) Entry {
	var e Entry
	if o, ok := ctx.Value(originKey{}).(origin); ok {
		e.IPAddress, e.UserAgent = o.ip, o.userAgent
	}
	if cid := logging.CorrelationID(ctx); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			e.CorrelationID = &id
		}
	}
	if identity, ok := auth.IdentityFromContext(ctx); ok {
		e.ActorSubject = identity.Subject
		e.ActorRoles = identity.Roles
		if actor, _, ok := auth.ActorUUID(ctx); ok {
			e.ActorID = actor
		}
		if len(identity.Scopes) > 0 {
			e.EntityContext = map[string]any{"scopes": identity.Scopes}
		}
	}
	return e
}

// Meta é açúcar para montar uma Entry a partir do contexto com ação,
// recurso e diff numa só chamada.
func Meta(ctx context.Context, action, resourceType, resourceID string, before, after any) Entry {
	e := FromContext(ctx)
	e.Action = action
	e.ResourceType = resourceType
	e.ResourceID = resourceID
	e.Before = before
	e.After = after
	return e
}
