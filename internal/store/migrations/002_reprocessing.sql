-- has_result: у матча есть посчитанный результат в match_players (сохраняется при неудачном пересчёте)
-- processed_version: версия обработки последней завершённой попытки (успешной или нет)
ALTER TABLE matches ADD COLUMN has_result INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN processed_version INTEGER NOT NULL DEFAULT 0;

UPDATE matches SET has_result = 1 WHERE status = 'done';
-- матчи, обработанные до появления версий, считаются обработанными версией 1
UPDATE matches SET processed_version = 1 WHERE status IN ('done', 'failed');
