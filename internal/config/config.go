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
}

const defaultMaxDemoSize = 1 << 30 // 1 ГБ

func FromEnv() (Config, error) {
	c := Config{
		DataDir:     getenv("DATA_DIR", "./data"),
		Addr:        getenv("ADDR", ":8080"),
		MaxDemoSize: defaultMaxDemoSize,
	}
	if v := os.Getenv("MAX_DEMO_SIZE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("MAX_DEMO_SIZE: ожидается положительное число байт, получено %q", v)
		}
		c.MaxDemoSize = n
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
