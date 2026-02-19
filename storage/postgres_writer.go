package storage

import (
	"database/sql"
	"fmt"
	"time"

	"airbnb-scraper/models"
	"airbnb-scraper/utils"

	_ "github.com/lib/pq"
)

// PostgresWriter handles storing clean listings in PostgreSQL
type PostgresWriter struct {
	db     *sql.DB
	logger *utils.Logger
}

// NewPostgresWriter creates a new PostgresWriter and pings the DB
func NewPostgresWriter(connStr string, logger *utils.Logger) (*PostgresWriter, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	logger.Info("Connected to PostgreSQL successfully")
	return &PostgresWriter{db: db, logger: logger}, nil
}

// CreateTable creates the alldata table if it doesn't exist, with indexes
func (w *PostgresWriter) CreateTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS alldata (
		id          SERIAL PRIMARY KEY,
		platform    VARCHAR(50)  NOT NULL,
		title       TEXT         NOT NULL,
		price       NUMERIC(10,2) DEFAULT 0,
		location    TEXT,
		rating      NUMERIC(4,2) DEFAULT 0,
		url         TEXT UNIQUE,
		description TEXT,
		scraped_at  TIMESTAMP    NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_alldata_price    ON alldata (price);
	CREATE INDEX IF NOT EXISTS idx_alldata_location ON alldata (location);
	CREATE INDEX IF NOT EXISTS idx_alldata_platform ON alldata (platform);
	CREATE INDEX IF NOT EXISTS idx_alldata_rating   ON alldata (rating);
	`
	_, err := w.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	w.logger.Info("Table 'alldata' is ready")
	return nil
}

// BatchInsert inserts clean listings in a single transaction, skipping duplicates
func (w *PostgresWriter) BatchInsert(listings []*models.Listing) error {
	if len(listings) == 0 {
		return nil
	}

	tx, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO alldata (platform, title, price, location, rating, url, description, scraped_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (url) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, l := range listings {
		_, err = stmt.Exec(
			l.Platform,
			l.Title,
			l.Price,
			l.Location,
			l.Rating,
			l.URL,
			l.Description,
			l.ScrapedAt,
		)
		if err != nil {
			w.logger.Warn("Skipping insert for '%s': %v", l.Title, err)
			continue
		}
		inserted++
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	w.logger.Info("Inserted %d/%d listings into PostgreSQL", inserted, len(listings))
	return nil
}

// Close closes the database connection
func (w *PostgresWriter) Close() {
	if w.db != nil {
		_ = w.db.Close()
	}
}