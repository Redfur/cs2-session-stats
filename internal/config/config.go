// Package config читает настройки сервиса из переменных окружения.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DataDir     string // каталог для БД и демок
	Addr        string // адрес HTTP-сервера
	MaxDemoSize int64  // лимит распакованного размера демки в байтах
	// RatingCalibration — сколько матчей нужно игроку, чтобы его OVR перестал быть калибровочным
	RatingCalibration int
}

const (
	defaultMaxDemoSize       = 1 << 30 // 1 ГБ
	defaultRatingCalibration = 10
)

func FromEnv() (Config, error) {
	c := Config{
		DataDir:     getenv("DATA_DIR", "./data"),
		Addr:        getenv("ADDR", ":8080"),
		MaxDemoSize: defaultMaxDemoSize,

		RatingCalibration: defaultRatingCalibration,
	}
	if v := os.Getenv("MAX_DEMO_SIZE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("MAX_DEMO_SIZE: ожидается положительное число байт, получено %q", v)
		}
		c.MaxDemoSize = n
	}
	if v := os.Getenv("RATING_CALIBRATION_MATCHES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return c, fmt.Errorf("RATING_CALIBRATION_MATCHES: ожидается целое число не меньше 1, получено %q", v)
		}
		c.RatingCalibration = n
	}
	return c, nil
}

func (c Config) DBPath() string   { return filepath.Join(c.DataDir, "cs2stats.db") }
func (c Config) DemosDir() string { return filepath.Join(c.DataDir, "demos") }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
