package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"airbnb-scraper/models"
	"airbnb-scraper/utils"
)

// CSVWriter handles writing raw listings to a CSV file
type CSVWriter struct {
	filePath string
	logger   *utils.Logger
}

// NewCSVWriter creates a new CSVWriter
func NewCSVWriter(filePath string, logger *utils.Logger) *CSVWriter {
	return &CSVWriter{filePath: filePath, logger: logger}
}

// WriteRawListings writes a slice of RawListings to CSV file
func (w *CSVWriter) WriteRawListings(listings []*models.RawListing) error {
	// Ensure output directory exists
	dir := filepath.Dir(w.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	file, err := os.Create(w.filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"platform", "title", "raw_price", "location",
		"raw_rating", "url", "description", "scraped_at",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, l := range listings {
		row := []string{
			l.Platform,
			l.Title,
			l.RawPrice,
			l.Location,
			l.RawRating,
			l.URL,
			l.Description,
			l.ScrapedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			w.logger.Error("Failed to write CSV row for '%s': %v", l.Title, err)
		}
	}

	w.logger.Info("Raw listings written to: %s (%d rows)", w.filePath, len(listings))
	return nil
}