package storage

import "airbnb-scraper/models"

// RawStorage defines the interface for storing raw scraped data
type RawStorage interface {
	SaveRaw(listings []*models.RawListing) error
	Close() error
}

// CleanStorage defines the interface for storing clean, normalized data
type CleanStorage interface {
	SaveClean(listings []*models.Listing) error
	Close() error
}
