-- данные для Round Swing: снаряжение команд на конец freeze time и время установки бомбы.
-- У существующих раундов 0 — данных нет; они появятся при пересчёте (result_version >= 4).
ALTER TABLE match_rounds ADD COLUMN equip_a INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_rounds ADD COLUMN equip_b INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_rounds ADD COLUMN plant_ms INTEGER NOT NULL DEFAULT 0; -- от начала демки; 0 — бомбу не установили
