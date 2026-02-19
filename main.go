package main

import (
	"fmt"
	"os"

	"airbnb-scraper/config"
	"airbnb-scraper/scraper/airbnb"
	"airbnb-scraper/services"
	"airbnb-scraper/storage"
	"airbnb-scraper/utils"
)

func main() {
	// ================== Bootstrap ====================
	logger := utils.NewLogger()
	cfg := config.Load()

	logger.Info("Airbnb Rental Scraping System")

	logger.Info("Properties per section: %d", cfg.PropertiesPerPage)
	logger.Info("Concurrency: %d | Rate delay: %dms | Retries: %d",
		cfg.MaxConcurrency, cfg.RateLimitDelay, cfg.MaxRetries)

	// =================== PostgreSQL Setup ========================================
	pgWriter, err := storage.NewPostgresWriter(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("Cannot connect to PostgreSQL: %v", err)
		logger.Error("Make sure Docker is running: docker start my-postgres")
		os.Exit(1)
	}
	defer pgWriter.Close()

	if err := pgWriter.CreateTable(); err != nil {
		logger.Error("Failed to create DB table: %v", err)
		os.Exit(1)
	}

	// =============== Scraping ===================================
	scraper := airbnb.NewAirbnbScraper(cfg, logger)
	rawListings, err := scraper.Scrape()
	if err != nil {
		logger.Error("Scraping failed: %v", err)
		os.Exit(1)
	}

	if len(rawListings) == 0 {
		logger.Warn("No listings scraped — check your network connection or Airbnb page structure")
		os.Exit(0)
	}

	// ========= CSV: store raw data ===========================
	csvWriter := storage.NewCSVWriter(cfg.CSVFilePath, logger)
	if err := csvWriter.WriteRawListings(rawListings); err != nil {
		logger.Error("Failed to write CSV: %v", err)
		// Non-fatal: continue to DB storage
	}

	// =========== Data Cleaning ======================
	cleaner := services.NewDataCleaner(logger)
	cleanListings := cleaner.Clean(rawListings)

	// ========= PostgreSQL: store clean data ============
	if err := pgWriter.BatchInsert(cleanListings); err != nil {
		logger.Error("Failed to insert into PostgreSQL: %v", err)
		os.Exit(1)
	}

	// ==== Insights ============================
	insightSvc := services.NewInsightService(logger)
	report := insightSvc.Generate(cleanListings)
	services.PrintInsightReport(report)

	fmt.Println(" Done! Raw data →", cfg.CSVFilePath)
	fmt.Println(" Clean data stored in PostgreSQL table: alldata")
}