package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/search"
)

var (
	ErrUnknownModule     = errors.New("kernel: módulo desconhecido")
	ErrCoreModule        = errors.New("kernel: módulo do núcleo não pode ser desativado")
	ErrDependencyMissing = errors.New("kernel: dependência desativada")
	ErrDependentActive   = errors.New("kernel: há módulos ativos que dependem deste")
	ErrAlreadyStarted    = errors.New("kernel: módulos só podem ser registrados antes do Start")
)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// Kernel mantém o registro de plugins e o estado de ativação de cada um.
type Kernel struct {
	store  Store
	logger *slog.Logger

	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string
	state   map[string]bool
	started bool
	loaded  bool

	subsMu sync.Mutex
	subs   []chan struct{}

	hooksMu sync.Mutex
	hooks   []func(ctx context.Context, key string, enabled bool)

	// pollInterval é o polling de segurança do Watch; min/maxBackoff, o
	// reinício de unidades supervisionadas que falham.
	pollInterval, minBackoff, maxBackoff time.Duration
}

// New cria o Kernel.
func New(store Store, logger *slog.Logger) *Kernel {
	return &Kernel{
		store:   store,
		logger:  logger,
		plugins: map[string]Plugin{},
		state:   map[string]bool{},

		pollInterval: 30 * time.Second,
		minBackoff:   time.Second,
		maxBackoff:   30 * time.Second,
	}
}

// RegisterModule registra um plugin no Kernel. Valida a chave, a unicidade
// e (no Start) as dependências declaradas.
func (k *Kernel) RegisterModule(p Plugin) error {
	m := p.Manifest()
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.started {
		return ErrAlreadyStarted
	}
	if !keyPattern.MatchString(m.Key) {
		return fmt.Errorf("kernel: chave de módulo inválida %q", m.Key)
	}
	if _, dup := k.plugins[m.Key]; dup {
		return fmt.Errorf("kernel: módulo %q registrado duas vezes", m.Key)
	}
	k.plugins[m.Key] = p
	k.order = append(k.order, m.Key)
	return nil
}

// MustRegister é RegisterModule que entra em pânico em erro de wiring
// (erro de programação detectado no boot).
func (k *Kernel) MustRegister(plugins ...Plugin) {
	for _, p := range plugins {
		if err := k.RegisterModule(p); err != nil {
			panic(err)
		}
	}
}

// Start valida o grafo de dependências, garante uma linha de estado por
// módulo e carrega o estado persistido.
func (k *Kernel) Start(ctx context.Context) error {
	k.mu.Lock()
	for _, key := range k.order {
		for _, dep := range k.plugins[key].Manifest().DependsOn {
			if _, ok := k.plugins[dep]; !ok {
				k.mu.Unlock()
				return fmt.Errorf("kernel: módulo %q depende de %q, que não foi registrado", key, dep)
			}
		}
	}
	if err := detectCycle(k.plugins); err != nil {
		k.mu.Unlock()
		return err
	}
	k.started = true
	keys := append([]string(nil), k.order...)
	k.mu.Unlock()

	for _, key := range keys {
		m := k.plugins[key].Manifest()
		if err := k.store.Ensure(ctx, key, m.Core || m.DefaultEnabled); err != nil {
			return err
		}
	}
	return k.Reload(ctx)
}

func detectCycle(plugins map[string]Plugin) error {
	const (
		white = iota
		grey
		black
	)
	color := map[string]int{}
	var visit func(string) error
	visit = func(k string) error {
		color[k] = grey
		for _, d := range plugins[k].Manifest().DependsOn {
			switch color[d] {
			case grey:
				return fmt.Errorf("kernel: dependência circular entre %q e %q", k, d)
			case white:
				if err := visit(d); err != nil {
					return err
				}
			}
		}
		color[k] = black
		return nil
	}
	for k := range plugins {
		if color[k] == white {
			if err := visit(k); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reload relê o estado persistido e dispara os hooks/assinantes para cada
// módulo cujo estado efetivo mudou.
func (k *Kernel) Reload(ctx context.Context) error {
	persisted, err := k.store.LoadAll(ctx)
	if err != nil {
		return err
	}
	var changed []string
	k.mu.Lock()
	// Compara o estado EFETIVO (que considera dependências) antes e
	// depois: desligar uma dependência também "desliga" quem depende dela.
	before := make(map[string]bool, len(k.plugins))
	for key := range k.plugins {
		before[key] = k.enabledLocked(key, 0)
	}
	for key, p := range k.plugins {
		m := p.Manifest()
		enabled := m.Core
		if !m.Core {
			v, ok := persisted[key]
			enabled = (ok && v) || (!ok && m.DefaultEnabled)
		}
		k.state[key] = enabled
	}
	if k.loaded {
		for key := range k.plugins {
			if k.enabledLocked(key, 0) != before[key] {
				changed = append(changed, key)
			}
		}
	}
	k.loaded = true
	k.mu.Unlock()

	sort.Strings(changed)
	for _, key := range changed {
		enabled := k.Enabled(key)
		k.logger.Info("kernel: estado de módulo alterado", slog.String("module", key), slog.Bool("enabled", enabled))
		k.fireHooks(ctx, key, enabled)
	}
	if len(changed) > 0 {
		k.notify()
	}
	return nil
}

// Enabled informa se o módulo está ativo AGORA. Um módulo só é
// considerado ativo se todas as suas dependências também estiverem.
func (k *Kernel) Enabled(key string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.enabledLocked(key, 0)
}

func (k *Kernel) enabledLocked(key string, depth int) bool {
	p, ok := k.plugins[key]
	if !ok || depth > 16 {
		return false
	}
	if !k.state[key] {
		return false
	}
	for _, dep := range p.Manifest().DependsOn {
		if !k.enabledLocked(dep, depth+1) {
			return false
		}
	}
	return true
}

// SetEnabled ativa/desativa um módulo, aplicando as regras do núcleo e de
// dependência, grava auditoria e propaga a mudança (NOTIFY -> todas as
// réplicas).
func (k *Kernel) SetEnabled(ctx context.Context, key string, enabled bool, actor string) error {
	k.mu.RLock()
	p, ok := k.plugins[key]
	if !ok {
		k.mu.RUnlock()
		return ErrUnknownModule
	}
	m := p.Manifest()
	if m.Core && !enabled {
		k.mu.RUnlock()
		return ErrCoreModule
	}
	if enabled {
		if missing := k.inactiveDepsLocked(m); len(missing) > 0 {
			k.mu.RUnlock()
			return fmt.Errorf("%w: ative antes %s", ErrDependencyMissing, quoteList(missing))
		}
	} else if active := k.activeDependentsLocked(key); len(active) > 0 {
		k.mu.RUnlock()
		return fmt.Errorf("%w: desative antes %s", ErrDependentActive, quoteList(active))
	}
	before := k.state[key]
	k.mu.RUnlock()

	entry := audit.FromContext(ctx)
	entry.Action = audit.ActionModuleToggled
	entry.ResourceType = "module"
	entry.ResourceID = key
	entry.Before = map[string]bool{"enabled": before}
	entry.After = map[string]bool{"enabled": enabled}
	if err := k.store.Set(ctx, key, enabled, actor, entry); err != nil {
		return err
	}
	return k.Reload(ctx)
}

// Watch escuta as mudanças de estado (NOTIFY) e faz polling de segurança
// a cada 30s. Bloqueia até ctx acabar.
func (k *Kernel) Watch(ctx context.Context) error {
	go func() {
		t := time.NewTicker(k.pollInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := k.Reload(ctx); err != nil && ctx.Err() == nil {
					k.logger.Warn("kernel: polling de estado falhou", slog.Any("error", err))
				}
			}
		}
	}()
	return k.store.Listen(ctx, func() {
		if err := k.Reload(ctx); err != nil && ctx.Err() == nil {
			k.logger.Warn("kernel: reload após NOTIFY falhou", slog.Any("error", err))
		}
	})
}

// OnChange registra um hook chamado a cada mudança de estado de módulo
// (ex.: derrubar assinaturas de WebSocket). Roda síncrono no Reload.
func (k *Kernel) OnChange(fn func(ctx context.Context, key string, enabled bool)) {
	k.hooksMu.Lock()
	k.hooks = append(k.hooks, fn)
	k.hooksMu.Unlock()
}

func (k *Kernel) fireHooks(ctx context.Context, key string, enabled bool) {
	k.hooksMu.Lock()
	hooks := append([]func(context.Context, string, bool){}, k.hooks...)
	k.hooksMu.Unlock()
	for _, h := range hooks {
		h(ctx, key, enabled)
	}
	k.mu.RLock()
	p := k.plugins[key]
	k.mu.RUnlock()
	if lc, ok := p.(Lifecycle); ok {
		var err error
		if enabled {
			err = lc.OnEnable(ctx)
		} else {
			err = lc.OnDisable(ctx)
		}
		if err != nil {
			k.logger.Error("kernel: hook de ciclo de vida falhou", slog.String("module", key), slog.Any("error", err))
		}
	}
}

// subscribe devolve um canal sinalizado a cada mudança de estado.
func (k *Kernel) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	k.subsMu.Lock()
	k.subs = append(k.subs, ch)
	k.subsMu.Unlock()
	return ch
}

func (k *Kernel) unsubscribe(ch chan struct{}) {
	k.subsMu.Lock()
	defer k.subsMu.Unlock()
	for i, c := range k.subs {
		if c == ch {
			k.subs = append(k.subs[:i], k.subs[i+1:]...)
			return
		}
	}
}

func (k *Kernel) notify() {
	k.subsMu.Lock()
	defer k.subsMu.Unlock()
	for _, ch := range k.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Plugins devolve os plugins na ordem de registro.
func (k *Kernel) Plugins() []Plugin {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]Plugin, 0, len(k.order))
	for _, key := range k.order {
		out = append(out, k.plugins[key])
	}
	return out
}

// ModuleStatus é a visão pública de um módulo.
type ModuleStatus struct {
	Manifest
	// Enabled é o estado EFETIVO: configurado como ativo e com todas as
	// dependências (transitivas) ativas.
	Enabled bool `json:"enabled"`
	// Configured é o estado desejado gravado pelo administrador (pode ser
	// true com Enabled=false se uma dependência estiver inativa).
	Configured bool `json:"configured"`
	// Dependents são os módulos que declaram DependsOn neste.
	Dependents []string `json:"dependents"`
	// BlockedBy são as dependências diretas inativas neste instante.
	BlockedBy []string `json:"blocked_by"`
}

// Status lista todos os módulos com o estado efetivo e o grafo de
// dependências nos dois sentidos.
func (k *Kernel) Status() []ModuleStatus {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]ModuleStatus, 0, len(k.order))
	for _, key := range k.order {
		m := k.plugins[key].Manifest()
		if m.DependsOn == nil {
			m.DependsOn = []string{}
		}
		if m.Permissions == nil {
			m.Permissions = []PermissionInfo{}
		}
		out = append(out, ModuleStatus{
			Manifest:   m,
			Enabled:    k.enabledLocked(key, 0),
			Configured: k.state[key],
			Dependents: k.dependentsLocked(key),
			BlockedBy:  k.inactiveDepsLocked(m),
		})
	}
	return out
}

// dependentsLocked lista (em ordem de registro) quem depende de key.
func (k *Kernel) dependentsLocked(key string) []string {
	out := []string{}
	for _, other := range k.order {
		for _, dep := range k.plugins[other].Manifest().DependsOn {
			if dep == key {
				out = append(out, other)
			}
		}
	}
	return out
}

// activeDependentsLocked: dependentes configurados como ativos (impedem
// desativar key).
func (k *Kernel) activeDependentsLocked(key string) []string {
	out := []string{}
	for _, d := range k.dependentsLocked(key) {
		if k.state[d] {
			out = append(out, d)
		}
	}
	return out
}

// inactiveDepsLocked: dependências diretas de m que não estão ativas.
func (k *Kernel) inactiveDepsLocked(m Manifest) []string {
	out := []string{}
	for _, dep := range m.DependsOn {
		if !k.enabledLocked(dep, 0) {
			out = append(out, dep)
		}
	}
	return out
}

func quoteList(keys []string) string {
	q := make([]string, len(keys))
	for i, k := range keys {
		q[i] = fmt.Sprintf("%q", k)
	}
	return strings.Join(q, ", ")
}

// Queues devolve todas as filas declaradas por plugins — declaradas no
// boot independentemente do estado, para que eventos se acumulem na fila
// durável enquanto o módulo está inativo.
func (k *Kernel) Queues() []messaging.QueueSpec {
	var out []messaging.QueueSpec
	for _, p := range k.Plugins() {
		if cp, ok := p.(ConsumerProvider); ok {
			for _, c := range cp.Consumers() {
				out = append(out, c.Queue)
			}
		}
	}
	return out
}

// SearchProviders devolve os providers dos módulos ATIVOS neste instante.
func (k *Kernel) SearchProviders() []search.Provider {
	var out []search.Provider
	for _, p := range k.Plugins() {
		sp, ok := p.(SearchProvider)
		if !ok || !k.Enabled(p.Manifest().Key) {
			continue
		}
		out = append(out, sp.SearchProviders()...)
	}
	return out
}
