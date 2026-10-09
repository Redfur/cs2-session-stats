package store

import "context"

// Матчи в очереди или в обработке повторно не ставятся: запрос пересчёта для них идемпотентен.
const requeueSet = `UPDATE matches SET status = 'pending', error = ''
	WHERE status NOT IN ('pending', 'parsing')`

// RequeueMatch ставит матч в очередь на пересчёт и возвращает его актуальное состояние.
func (s *Store) RequeueMatch(ctx context.Context, id int64) (Match, error) {
	if _, err := s.db.ExecContext(ctx, requeueSet+" AND id = ?", id); err != nil {
		return Match{}, err
	}
	return s.GetMatch(ctx, id)
}

// RequeueSession ставит в очередь все матчи сессии и возвращает число поставленных.
// Существование сессии проверяет вызывающий код.
func (s *Store) RequeueSession(ctx context.Context, sessionID int64) (int64, error) {
	return s.requeue(ctx, requeueSet+" AND session_id = ?", sessionID)
}

// RequeueAll ставит в очередь все матчи.
func (s *Store) RequeueAll(ctx context.Context) (int64, error) {
	return s.requeue(ctx, requeueSet)
}

// RequeueOutdated ставит в очередь матчи, последняя обработка которых выполнена версией ниже version.
// Новые матчи (processed_version = 0) уже стоят в очереди и не затрагиваются.
func (s *Store) RequeueOutdated(ctx context.Context, version int) (int64, error) {
	return s.requeue(ctx, requeueSet+" AND processed_version < ?", version)
}

func (s *Store) requeue(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
