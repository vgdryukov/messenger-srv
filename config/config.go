package config

import (
	"os"
	"time"
)

type Config struct {
	Environment string       `json:"environment"`
	Server      ServerConfig `json:"server"`
	Storage     Storage      `json:"storage"`
	RateLimit   RateLimit    `json:"rate_limit"`
}

type ServerConfig struct {
	Host           string        `json:"host"`
	Port           string        `json:"port"`
	MaxConnections int           `json:"max_connections"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
}

type Storage struct {
	UsersFile    string `json:"users_file"`
	MessagesFile string `json:"messages_file"`
	ContactsFile string `json:"contacts_file"`
	GroupsFile   string `json:"groups_file"`
}

type RateLimit struct {
	MaxAttempts   int           `json:"max_attempts"`
	WindowSize    time.Duration `json:"window_size"`
	MessageLimit  int           `json:"message_limit"`
	MessageWindow time.Duration `json:"message_window"`
}

func Load() (*Config, error) {
	// Значения по умолчанию
	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Server: ServerConfig{
			Host:           getEnv("HOST", "localhost"),
			Port:           getEnv("PORT", "8080"),
			MaxConnections: 1000,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
		},
		Storage: Storage{
			UsersFile:    getEnv("USERS_FILE", "users.dat"),
			MessagesFile: getEnv("MESSAGES_FILE", "messages.dat"),
			ContactsFile: getEnv("CONTACTS_FILE", "contacts.dat"),
			GroupsFile:   getEnv("GROUPS_FILE", "groups.dat"),
		},
		RateLimit: RateLimit{
			MaxAttempts:   5,
			WindowSize:    5 * time.Minute,
			MessageLimit:  30,
			MessageWindow: 1 * time.Minute,
		},
	}

	// В production слушаем все интерфейсы
	if cfg.Environment == "production" {
		cfg.Server.Host = "0.0.0.0"
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
