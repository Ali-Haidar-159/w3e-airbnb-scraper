package config

import (
	"os"
	"strconv"
)

// Config holds all application-level configuration
type Config struct {
	// Database
	DatabaseURL string

	// Scraper
	MaxConcurrency    int
	RateLimitDelay    int // milliseconds between requests
	MaxRetries        int
	PropertiesPerPage int // how many properties to scrape per location section

	// Output
	CSVFilePath string

	// Airbnb
	AirbnbURL string
}

// Load reads configuration from environment variables or falls back to defaults
func Load() *Config {
	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://ali:1234@localhost:5432/mydb?sslmode=disable"),
		MaxConcurrency:    getEnvInt("MAX_CONCURRENCY", 3),
		RateLimitDelay:    getEnvInt("RATE_LIMIT_DELAY_MS", 2000),
		MaxRetries:        getEnvInt("MAX_RETRIES", 3),
		PropertiesPerPage: getEnvInt("PROPERTIES_PER_SECTION", 5),
		CSVFilePath:       getEnv("CSV_FILE_PATH", "output/raw_listings.csv"),
		AirbnbURL:         getEnv("AIRBNB_URL", "https://www.airbnb.com"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}