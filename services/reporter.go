package services

import (
	"fmt"
	"sort"
	"strings"

	"airbnb-scraper/models"
)

// PrintInsightReport formats and prints the insight report to terminal
func PrintInsightReport(report *models.InsightReport) {
	border := strings.Repeat("═", 55)
	thin := strings.Repeat("─", 55)

	fmt.Printf("\n╔%s╗\n", border)
	fmt.Printf("║%s║\n", center("VACATION RENTAL MARKET INSIGHTS ", 55))
	fmt.Printf("╚%s╝\n", border)

	fmt.Printf("\n OVERVIEW\n%s\n", thin)
	fmt.Printf("  Total Listings Scraped  : %d\n", report.TotalListings)
	fmt.Printf("  Airbnb Listings         : %d\n", report.AirbnbListings)
	fmt.Printf("  Average Price/Night     : $%.2f\n", report.AveragePrice)
	fmt.Printf("  Minimum Price/Night     : $%.2f\n", report.MinPrice)
	fmt.Printf("  Maximum Price/Night     : $%.2f\n", report.MaxPrice)

	if report.MostExpensive != nil {
		fmt.Printf("\n MOST EXPENSIVE PROPERTY\n%s\n", thin)
		fmt.Printf("  Title    : %s\n", report.MostExpensive.Title)
		fmt.Printf("  Price    : $%.2f/night\n", report.MostExpensive.Price)
		fmt.Printf("  Location : %s\n", report.MostExpensive.Location)
		fmt.Printf("  URL      : %s\n", report.MostExpensive.URL)
	}

	if len(report.ListingsByLocation) > 0 {
		fmt.Printf("\n LISTINGS PER LOCATION\n%s\n", thin)
		// Sort by count descending
		type locCount struct {
			loc   string
			count int
		}
		var locs []locCount
		for loc, cnt := range report.ListingsByLocation {
			locs = append(locs, locCount{loc, cnt})
		}
		sort.Slice(locs, func(i, j int) bool {
			return locs[i].count > locs[j].count
		})
		for _, lc := range locs {
			bar := strings.Repeat("▓", lc.count)
			fmt.Printf("  %-25s %3d  %s\n", lc.loc+":", lc.count, bar)
		}
	}

	if len(report.TopRated) > 0 {
		fmt.Printf("\n TOP %d HIGHEST RATED PROPERTIES\n%s\n", len(report.TopRated), thin)
		for i, l := range report.TopRated {
			fmt.Printf("  %d. %-35s %.2f \n", i+1, truncate(l.Title, 35), l.Rating)
		}
	}

	fmt.Printf("\n%s\n\n", border)
}

func center(s string, width int) string {
	// Account for possible emoji width
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	pad := (width - len(runes)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-len(runes)-pad)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}