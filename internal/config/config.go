package config

import (
	"os"
	"strconv"
)

type Limiter struct {
	RPS     float64 // request per second
	Burst   int // how many requests allowed at once
	Enabled bool // whether rate limiting is turned on
}

type Config struct {
	Env         string
	ServerAddr  string
	DatabaseURL string
	JWTSecret   string
	Limiter
}

func Load() *Config {
	rps, _ := strconv.ParseFloat(getEnv("LIMITER_RPS", "2"), 64)
	burst, _ := strconv.Atoi(getEnv("LIMITER_BURST", "4"))
	enabled, _ := strconv.ParseBool(getEnv("LIMITER_ENABLED", "true"))

	appLimiter := Limiter{
		RPS: rps,
		Burst: burst,
		Enabled: enabled,
	}
	
	return &Config{
		Env:         getEnv("ENV", "development"),
		ServerAddr: getEnv("SERVER_ADDRESS", ":4000"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:nur1dd1n@localhost:5432/worktime?sslmode=disable"),
		JWTSecret: getEnv("JWT_SECRET", "e818f561410d5d126a48f229214f6b7d37a0cc51b55a9148507eef46"),
		Limiter: appLimiter,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}
