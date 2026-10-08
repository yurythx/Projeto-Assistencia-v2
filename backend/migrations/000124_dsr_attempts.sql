-- +goose Up
-- data_subject_requests.attempts — a eliminação (LGPD art. 18, VI) passa a
-- ser retentada numa falha transitória; antes, a primeira falha marcava a
-- solicitação como 'failed' prometendo um reprocessamento que não existia.
ALTER TABLE data_subject_requests ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE data_subject_requests DROP COLUMN attempts;
