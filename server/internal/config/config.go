package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	DatabasePath          string
	MigrationsDir         string
	JWTSecret             string
	PasswordPepper        string
	TrustedProxyCIDRs     string
	DatabaseEncryptionKey string
	CORSOrigin            string
}

func Load() Config {
	_ = godotenv.Load(".env")
	return Config{
		Port:                  getEnv("PORT", "8080"),
		DatabasePath:          getEnv("DATABASE_PATH", "./data/smsrelay.db"),
		MigrationsDir:         getEnv("MIGRATIONS_DIR", ""),
		JWTSecret:             getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		PasswordPepper:        getEnv("PASSWORD_PEPPER", ""),
		TrustedProxyCIDRs:     getEnv("TRUSTED_PROXY_CIDRS", ""),
		DatabaseEncryptionKey: getEnv("DATABASE_ENCRYPTION_KEY", ""),
		CORSOrigin:            getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c Config) Addr() string {
	port, _ := strconv.Atoi(c.Port)
	if port == 0 {
		port = 8080
	}
	return ":" + strconv.Itoa(port)
}
