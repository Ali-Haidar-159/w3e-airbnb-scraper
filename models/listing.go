package models

import "time"

// RawListing represents unprocessed data scraped directly from Airbnb
type RawListing struct {
	Platform    string
	Title       string
	RawPrice    string // e.g. "$71 for 2 nights"
	Location    string
	RawRating   string // e.g. "4.82"
	URL         string
	Description string
	ScrapedAt   time.Time
}

// Listing represents a cleaned, normalized listing ready for DB storage
type Listing struct {
	ID          int64
	Platform    string
	Title       string
	Price       float64 // price per night normalized
	Location    string
	Rating      float64
	URL         string
	Description string
	ScrapedAt   time.Time
}

// InsightReport holds computed analytics from the final dataset
type InsightReport struct {
	TotalListings      int
	AirbnbListings     int
	AveragePrice       float64
	MinPrice           float64
	MaxPrice           float64
	MostExpensive      *Listing
	TopRated           []*Listing
	ListingsByLocation map[string]int
}