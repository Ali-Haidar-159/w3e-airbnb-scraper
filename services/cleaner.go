package services

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"airbnb-scraper/models"
	"airbnb-scraper/utils"
)

var (
	priceRegex  = regexp.MustCompile(`\$?([\d,]+(?:\.\d{2})?)`)
	ratingRegex = regexp.MustCompile(`([45]\.\d{1,2}|\d\.\d{1,2})`)
)

// DataCleaner normalizes raw scraped data into clean Listing records
type DataCleaner struct {
	logger *utils.Logger
}

// NewDataCleaner creates a new DataCleaner
func NewDataCleaner(logger *utils.Logger) *DataCleaner {
	return &DataCleaner{logger: logger}
}

// Clean converts a slice of RawListings to clean Listings
func (c *DataCleaner) Clean(raw []*models.RawListing) []*models.Listing {
	seen := make(map[string]bool)
	var cleaned []*models.Listing

	for _, r := range raw {
		// Skip if title or URL empty
		if strings.TrimSpace(r.Title) == "" {
			c.logger.Debug("Skipping listing with empty title")
			continue
		}

		// Deduplicate by URL
		key := strings.TrimSpace(r.URL)
		if key == "" {
			key = strings.TrimSpace(r.Title) + "|" + strings.TrimSpace(r.Location)
		}
		if seen[key] {
			c.logger.Debug("Skipping duplicate: %s", r.Title)
			continue
		}
		seen[key] = true

		price := parsePrice(r.RawPrice)
		rating := parseRating(r.RawRating)

		listing := &models.Listing{
			Platform:    strings.TrimSpace(r.Platform),
			Title:       strings.TrimSpace(r.Title),
			Price:       price,
			Location:    cleanLocation(r.Location),
			Rating:      rating,
			URL:         strings.TrimSpace(r.URL),
			Description: strings.TrimSpace(r.Description),
			ScrapedAt:   r.ScrapedAt,
		}
		if listing.ScrapedAt.IsZero() {
			listing.ScrapedAt = time.Now()
		}

		cleaned = append(cleaned, listing)
	}

	c.logger.Info("Cleaned %d listings from %d raw records", len(cleaned), len(raw))
	return cleaned
}

// parsePrice extracts a per-night price number from a raw string like "$71 for 2 nights"
func parsePrice(raw string) float64 {
	if raw == "" {
		return 0
	}
	// Remove commas
	cleaned := strings.ReplaceAll(raw, ",", "")

	// Find first number after $
	matches := priceRegex.FindStringSubmatch(cleaned)
	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	// If "for N nights", divide to get per-night
	nightsRegex := regexp.MustCompile(`for\s+(\d+)\s+night`)
	if m := nightsRegex.FindStringSubmatch(cleaned); len(m) >= 2 {
		nights, err := strconv.ParseFloat(m[1], 64)
		if err == nil && nights > 0 {
			return val / nights
		}
	}

	return val
}

// parseRating extracts a float rating from strings like "4.82 out of 5 average rating"
func parseRating(raw string) float64 {
	if raw == "" {
		return 0
	}
	matches := ratingRegex.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	// Sanity: rating should be between 0 and 5
	if val > 5 {
		return 0
	}
	return val
}

// cleanLocation normalizes location strings
func cleanLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	// Remove prefix patterns like "Condo in " from location-as-title extractions
	if idx := strings.LastIndex(loc, " in "); idx != -1 && idx < 30 {
		loc = loc[idx+4:]
	}
	return loc
}