package services

import (
	"sort"

	"airbnb-scraper/models"
	"airbnb-scraper/utils"
)

// InsightService computes analytics from the cleaned dataset
type InsightService struct {
	logger *utils.Logger
}

// NewInsightService creates a new InsightService
func NewInsightService(logger *utils.Logger) *InsightService {
	return &InsightService{logger: logger}
}

// Generate computes all required insights from a slice of clean listings
func (s *InsightService) Generate(listings []*models.Listing) *models.InsightReport {
	report := &models.InsightReport{
		ListingsByLocation: make(map[string]int),
	}

	if len(listings) == 0 {
		s.logger.Warn("No listings to generate insights from")
		return report
	}

	var totalPrice float64
	report.MinPrice = listings[0].Price
	report.MaxPrice = listings[0].Price

	for _, l := range listings {
		// Counts
		report.TotalListings++
		if l.Platform == "Airbnb" {
			report.AirbnbListings++
		}

		// Price stats
		if l.Price > 0 {
			totalPrice += l.Price
			if l.Price < report.MinPrice || report.MinPrice == 0 {
				report.MinPrice = l.Price
			}
			if l.Price > report.MaxPrice {
				report.MaxPrice = l.Price
				report.MostExpensive = l
			}
		}

		// Location count
		if l.Location != "" {
			report.ListingsByLocation[l.Location]++
		}
	}

	// Average price
	if report.TotalListings > 0 {
		report.AveragePrice = totalPrice / float64(report.TotalListings)
	}

	// If MostExpensive not set (all prices 0), just pick first
	if report.MostExpensive == nil && len(listings) > 0 {
		report.MostExpensive = listings[0]
	}

	// Top 5 highest-rated
	rated := make([]*models.Listing, 0, len(listings))
	for _, l := range listings {
		if l.Rating > 0 {
			rated = append(rated, l)
		}
	}
	sort.Slice(rated, func(i, j int) bool {
		return rated[i].Rating > rated[j].Rating
	})
	maxTop := 5
	if len(rated) < maxTop {
		maxTop = len(rated)
	}
	report.TopRated = rated[:maxTop]

	return report
}