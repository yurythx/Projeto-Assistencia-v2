package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// wormInterval é de quanto em quanto tempo o worker verifica se há dias
// completos ainda não exportados. Diário seria suficiente; 6h dá margem
// para o worker ter ficado fora do ar por um tempo sem acumular atraso.
const wormInterval = 6 * time.Hour

// wormLockKey identifica o advisory lock da exportação WORM: com várias
// réplicas do worker, só uma exporta cada dia (as outras desistem até o
// próximo tick). Sem isso, duas réplicas liam a mesma marca d'água e
// gravavam o mesmo dia duas vezes — duas versões imutáveis com digests
// diferentes no bucket, e uma das marcas d'água falhando.
const wormLockKey int64 = 0x4e58_574f_524d // "NXWORM"

// txBeginner é o banco da exportação: o pool em produção (Begin abre uma
// transação por dia exportado).
type txBeginner interface {
	database.DBTX
	Begin(context.Context) (pgx.Tx, error)
}

type wormExporter struct {
	db            txBeginner
	store         storage.Provider
	worm          storage.WORMWriter // != nil quando store suporta object-lock
	bucket        string
	retentionDays int
	logger        *slog.Logger
	bucketReady   bool
	interval      time.Duration
	now           func() time.Time
}

// WORMExporter é um processor do worker. Serializa cada dia COMPLETO de audit_logs ainda não
// exportado, encadeia o SHA-256 ao do dia anterior (evidência de
// adulteração) e sobe arquivo + digest para o object storage. F2.6.
//
// Se o provider implementa storage.WORMWriter, o bucket é criado com
// object-lock e cada objeto recebe retenção Compliance de retentionDays;
// caso contrário, cai para Put comum e registra o aviso (a cadeia de
// hash ainda dá evidência de adulteração).
func WORMExporter(pool *pgxpool.Pool, store storage.Provider, bucket string, retentionDays int, logger *slog.Logger) func(ctx context.Context) error {
	return newWORMExporter(pool, store, bucket, retentionDays, logger).loop
}

func newWORMExporter(db txBeginner, store storage.Provider, bucket string, retentionDays int, logger *slog.Logger) *wormExporter {
	e := &wormExporter{db: db, store: store, bucket: bucket, retentionDays: retentionDays, logger: logger,
		interval: wormInterval, now: time.Now}
	if w, ok := store.(storage.WORMWriter); ok {
		e.worm = w
	}
	return e
}

func (e *wormExporter) loop(ctx context.Context) error {
	e.run(ctx)
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			e.run(ctx)
		}
	}
}

func (e *wormExporter) ensureBucket(ctx context.Context) bool {
	if e.bucketReady {
		return true
	}
	if e.worm != nil {
		if err := e.worm.EnsureImmutableBucket(ctx, e.bucket, e.retentionDays); err != nil {
			e.logger.Error("audit worm: não foi possível preparar o bucket imutável — object-lock indisponível; a cadeia de hash segue como evidência",
				slog.String("bucket", e.bucket), slog.Any("error", err))
			e.worm = nil // cai para Put comum daqui pra frente
		}
	}
	if e.worm == nil {
		if be, ok := e.store.(interface {
			EnsureBucket(context.Context, string) error
		}); ok {
			if err := be.EnsureBucket(ctx, e.bucket); err != nil {
				e.logger.Error("audit worm: não foi possível garantir o bucket", slog.Any("error", err))
				return false
			}
		}
		e.logger.Warn("audit worm: bucket SEM object-lock — configure um bucket dedicado com lock na infraestrutura para a garantia WORM completa",
			slog.String("bucket", e.bucket))
	}
	e.bucketReady = true
	return true
}

// run exporta, em ordem, todos os dias completos (até ontem, UTC) ainda
// não exportados. As exportações são sempre contíguas — um dia só sai
// depois do anterior —, então cada passo retoma do dia seguinte ao último
// exportado (a marca d'água), sem revisitar todo o histórico a cada tick.
func (e *wormExporter) run(ctx context.Context) {
	if !e.ensureBucket(ctx) {
		return
	}
	for {
		done, err := e.exportNext(ctx)
		if err != nil {
			// tenta de novo no próximo tick; a cadeia precisa ser sequencial
			e.logger.Error("audit worm: falha ao exportar", slog.Any("error", err))
			return
		}
		if done {
			return
		}
	}
}

// exportNext exporta o próximo dia pendente numa transação própria, sob o
// advisory lock: a marca d'água é relida já com o lock, então uma réplica
// que chegou depois vê o dia exportado e não o grava de novo. done indica
// que não há (ou não cabe a esta réplica) mais nada neste tick.
func (e *wormExporter) exportNext(ctx context.Context) (done bool, err error) {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, wormLockKey).Scan(&locked); err != nil {
		return false, fmt.Errorf("advisory lock: %w", err)
	}
	if !locked {
		e.logger.Debug("audit worm: outra réplica está exportando; tenta no próximo tick")
		return true, nil
	}
	day, prevSHA, err := e.nextDay(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("marca d'água: %w", err)
	}
	yesterday := e.now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	if day.IsZero() || day.After(yesterday) {
		return true, nil // trilha vazia ou em dia
	}
	res, err := e.exportOneDay(ctx, tx, day, prevSHA)
	if err != nil {
		return false, fmt.Errorf("dia %s: %w", day.Format("2006-01-02"), err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit do dia %s: %w", day.Format("2006-01-02"), err)
	}
	e.logger.Info("audit worm: dia exportado", slog.String("day", day.Format("2006-01-02")))

	// Fora da transação: uma falha aqui não pode desfazer a marca d'água
	// de um dia cujos objetos imutáveis já foram gravados.
	if err := NewWriter(e.db).Record(ctx, Entry{
		Action:       "audit.worm.exported",
		ResourceType: "audit_worm_exports",
		ResourceID:   day.Format("2006-01-02"),
		Metadata: map[string]any{
			"rows": res.rows, "sha256": res.sha256, "object_key": res.objectKey,
			"bucket": e.bucket, "immutable": e.worm != nil,
		},
	}); err != nil {
		e.logger.Warn("audit worm: falha ao registrar a exportação na trilha", slog.Any("error", err))
	}
	return false, nil
}

// nextDay devolve o primeiro dia ainda não exportado e o SHA-256 do último
// exportado (ao qual o próximo se encadeia). Sem exportação alguma, começa
// no dia do registro mais antigo; trilha vazia devolve o tempo zero.
func (e *wormExporter) nextDay(ctx context.Context, db database.DBTX) (time.Time, string, error) {
	var last time.Time
	var sha string
	err := db.QueryRow(ctx, `SELECT day, sha256 FROM audit_worm_exports ORDER BY day DESC LIMIT 1`).Scan(&last, &sha)
	if err == nil {
		return last.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour), sha, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, "", err
	}
	var earliest *time.Time
	if err := db.QueryRow(ctx, `SELECT min(created_at) FROM audit_logs`).Scan(&earliest); err != nil {
		return time.Time{}, "", err
	}
	if earliest == nil {
		return time.Time{}, "", nil
	}
	return earliest.UTC().Truncate(24 * time.Hour), "", nil
}

func (e *wormExporter) put(ctx context.Context, key string, body []byte, contentType string) error {
	if e.worm != nil {
		return e.worm.PutImmutable(ctx, e.bucket, key, bytes.NewReader(body), int64(len(body)), contentType, e.retentionDays)
	}
	return e.store.Put(ctx, e.bucket, key, bytes.NewReader(body), int64(len(body)), contentType)
}

// dayExport resume um dia exportado (para a trilha de auditoria).
type dayExport struct {
	rows      int
	sha256    string
	objectKey string
}

// exportOneDay grava o arquivo do dia (encadeado a prevSHA), o digest e a
// marca d'água (em tx).
func (e *wormExporter) exportOneDay(ctx context.Context, tx database.DBTX, day time.Time, prevSHA string) (dayExport, error) {
	next := day.Add(24 * time.Hour)
	rows, err := tx.Query(ctx, `
		SELECT id, COALESCE(actor_id::text,''), action, COALESCE(resource_type,''),
		       COALESCE(resource_id,''), COALESCE(metadata::text,'{}'),
		       COALESCE(correlation_id::text,''), COALESCE(host(ip_address),''), created_at,
		       chain_pos, prev_hash, hash
		FROM audit_logs
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY chain_pos ASC`, day, next)
	if err != nil {
		return dayExport{}, fmt.Errorf("query day: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	header, _ := json.Marshal(map[string]any{
		"_worm_header": true,
		"day":          day.Format("2006-01-02"),
		"prev_sha256":  prevSHA,
		"generated_at": e.now().UTC().Format(time.RFC3339),
	})
	buf.Write(header)
	buf.WriteByte('\n')

	count := 0
	for rows.Next() {
		var id, actorID, action, rType, rID, meta, corr, ip, prevHash, hash string
		var createdAt time.Time
		var chainPos int64
		if err := rows.Scan(&id, &actorID, &action, &rType, &rID, &meta, &corr, &ip, &createdAt, &chainPos, &prevHash, &hash); err != nil {
			return dayExport{}, fmt.Errorf("scan: %w", err)
		}
		// O hash da cadeia vai junto em cada linha: a cópia WORM permite
		// provar, fora do banco, que a sequência não foi alterada.
		line, _ := json.Marshal(map[string]any{
			"id": id, "actor_id": actorID, "action": action, "resource_type": rType,
			"resource_id": rID, "metadata": json.RawMessage(meta), "correlation_id": corr,
			"ip_address": ip, "created_at": createdAt.UTC().Format(time.RFC3339Nano),
			"chain_pos": chainPos, "prev_hash": prevHash, "hash": hash,
		})
		buf.Write(line)
		buf.WriteByte('\n')
		count++
	}
	if rows.Err() != nil {
		return dayExport{}, fmt.Errorf("rows: %w", rows.Err())
	}

	sum := sha256.Sum256(buf.Bytes())
	digest := hex.EncodeToString(sum[:])
	objectKey := fmt.Sprintf("%s/%s.jsonl", day.Format("2006/01"), day.Format("2006-01-02"))

	if err := e.put(ctx, objectKey, buf.Bytes(), "application/x-ndjson"); err != nil {
		return dayExport{}, fmt.Errorf("put object: %w", err)
	}
	digestBody := []byte(digest + "  " + day.Format("2006-01-02") + ".jsonl\n")
	if err := e.put(ctx, objectKey+".sha256", digestBody, "text/plain"); err != nil {
		return dayExport{}, fmt.Errorf("put digest: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_worm_exports (day, row_count, sha256, prev_sha256, object_key)
		VALUES ($1, $2, $3, $4, $5)`, day, count, digest, prevSHA, objectKey); err != nil {
		return dayExport{}, fmt.Errorf("insert watermark: %w", err)
	}
	return dayExport{rows: count, sha256: digest, objectKey: objectKey}, nil
}
