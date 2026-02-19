# Airbnb Rental Market Scraping System

A production-grade, concurrent web scraping system built in **Go** using the `chromedp` library. The system scrapes public rental listing data from [Airbnb](https://www.airbnb.com), processes and cleans the data, stores raw output to CSV and clean data to PostgreSQL, then prints a structured market insight report to the terminal.

---

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
  - [1. Clone the Repository](#1-clone-the-repository)
  - [2. Install Go](#2-install-go)
  - [3. Set Up PostgreSQL via Docker](#4-set-up-postgresql-via-docker)
  - [4. Install Go Dependencies](#5-install-go-dependencies)
- [Configuration](#configuration)
- [Running the Project](#running-the-project)
- [Expected Output](#expected-output)
- [Database Schema](#database-schema)
- [Verifying the Data](#verifying-the-data)

---

## Project Overview

This system automates the collection of vacation rental data from Airbnb's homepage. It:

1. Launches a headless Chromium browser and navigates to `airbnb.com`
2. Discovers all location sections (e.g. *"Available next month in Bangkok"*, *"Popular homes in Kuala Lumpur"*)
3. Scrapes **5 properties per location section**, paginating to the next page when needed
4. Visits individual property detail pages to enrich listings with descriptions
5. Saves **raw (uncleaned) data** to `output/raw_listings.csv`
6. **Normalizes and deduplicates** the data (e.g. `"$71 for 2 nights"` → `35.50` per night)
7. Stores **clean data** in a PostgreSQL table named `alldata`
8. Prints a **market insight report** to the terminal

---

## Architecture

```
airbnb-scraper/
├── config/               # Environment-based configuration loader
├── models/               # Data structs: RawListing, Listing, InsightReport
├── scraper/
│   └── airbnb/           # AirbnbScraper — chromedp browser automation
├── storage/
│   ├── csv_writer.go     # Writes raw listings to CSV
│   └── postgres.go       # Batch inserts clean listings into PostgreSQL
├── services/
│   ├── cleaner.go        # Normalizes and deduplicates raw data
│   ├── insights.go       # Computes market analytics
│   └── reporter.go       # Formats and prints the terminal report
├── utils/
│   ├── logger.go         # Leveled logger (INFO / WARN / ERROR / DEBUG)
│   ├── ratelimiter.go    # Thread-safe rate limiter between requests
│   └── retry.go          # Exponential backoff retry logic
├── output/               # Auto-created at runtime; stores raw_listings.csv
├── main.go               # Composition root — wires all components
├── go.mod
└── README.md
```

**Design principles applied:** Single Responsibility, Dependency Injection, no global state, thread-safe shared data (sync.Mutex), fully configurable via environment variables.

---

## Tech Stack

| Component        | Technology                        |
|------------------|-----------------------------------|
| Language         | Go 1.21+                          |
| Browser Control  | chromedp v0.9.3                   |
| Database Driver  | lib/pq v1.10.9 (PostgreSQL)       |
| Database         | PostgreSQL 16 (via Docker)        |
| Raw Data Storage | CSV (encoding/csv — stdlib)       |

---

## Prerequisites

Make sure the following are installed on your machine before proceeding:

| Requirement | Minimum Version | Verify With          |
|-------------|-----------------|----------------------|
| Go          | 1.21            | `go version`         |
| Docker      | any             | `docker --version`   |
| Chromium    | any             | `chromium --version` |
| Git         | any             | `git --version`      |

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/Ali-Haidar-159/w3e-airbnb-scraper.git
cd w3e-airbnb-scraper
```

---

### 2. Install Go

**Ubuntu / Debian (quick):**
```bash
sudo apt update && sudo apt install -y golang-go
go version
```

---

### 3. Set Up PostgreSQL via Docker

**First time — create the container:**
```bash
docker run --name my-postgres \
  -e POSTGRES_USER=ali \
  -e POSTGRES_PASSWORD=1234 \
  -e POSTGRES_DB=mydb \
  -p 5432:5432 \
  -v pgdata:/var/lib/postgresql/data \
  -d postgres:16
```

**Every subsequent time — just start it:**
```bash
docker start my-postgres
```

**Verify it is running:**
```bash
docker ps 
```

You should see the container listed with status `Up`.

> The application automatically creates the `alldata` table on first run. No manual SQL is required.

---

### 5. Install Go Dependencies

```bash
go mod tidy
```

This downloads:
- `github.com/chromedp/chromedp v0.9.3` — headless Chromium browser control
- `github.com/lib/pq v1.10.9` — PostgreSQL driver

---

## Configuration

All settings have working defaults. Override any of them by exporting environment variables before running.

| Environment Variable     | Default                                                   | Description                                |
|--------------------------|-----------------------------------------------------------|--------------------------------------------|
| `DATABASE_URL`           | `postgres://ali:1234@localhost:5432/mydb?sslmode=disable` | PostgreSQL connection string               |
| `MAX_CONCURRENCY`        | `3`                                                       | Max parallel goroutines for scraping       |
| `RATE_LIMIT_DELAY_MS`    | `2000`                                                    | Milliseconds to wait between requests      |
| `MAX_RETRIES`            | `3`                                                       | Retry attempts on page load failure        |
| `PROPERTIES_PER_SECTION` | `5`                                                       | Properties to collect per location section |
| `CSV_FILE_PATH`          | `output/raw_listings.csv`                                 | Output path for raw CSV file               |
| `AIRBNB_URL`             | `https://www.airbnb.com`                                  | Airbnb base URL                            |

---

## Running the Project

### Standard run (recommended defaults):
```bash
go run main.go
```

### Run with custom environment variables:
```bash
DATABASE_URL="postgres://ali:1234@localhost:5432/mydb?sslmode=disable" \
PROPERTIES_PER_SECTION=5 \
RATE_LIMIT_DELAY_MS=2500 \
MAX_RETRIES=3 \
go run main.go
```

### Build a binary and run it:
```bash
go build -o airbnb-scraper .
./airbnb-scraper
```

> **Expected runtime:** 3–8 minutes depending on how many location sections Airbnb is showing and your network speed.

---

## Expected Output

### Terminal insight report (printed at the end of a successful run):

```

          VACATION RENTAL MARKET INSIGHTS       


OVERVIEW
───────────────────────────────────────────────────────
  Total Listings Scraped  : 45
  Airbnb Listings         : 45
  Average Price/Night     : $52.30
  Minimum Price/Night     : $20.50
  Maximum Price/Night     : $112.00

MOST EXPENSIVE PROPERTY
───────────────────────────────────────────────────────
  Title    : Apartment in Khet Ratchathewi
  Price    : $112.00/night
  Location : Khet Ratchathewi
  URL      : https://www.airbnb.com/rooms/...

LISTINGS PER LOCATION
───────────────────────────────────────────────────────
  Bangkok:                   5  
  Kuala Lumpur:              5  
  Seoul:                     5  

TOP 5 HIGHEST RATED PROPERTIES
───────────────────────────────────────────────────────
  1. Apartment in Khet Ratchathewi         4.96 
  2. Place to stay in Bang Na              5.00 
  3. Room in Khet Huai Khwang              4.96 

Done! Raw data → output/raw_listings.csv
Clean data stored in PostgreSQL table: alldata
```

### Files produced:

| Output                       | Description                                    |
|------------------------------|------------------------------------------------|
| `output/raw_listings.csv`    | Raw scraped data exactly as seen on the page   |
| PostgreSQL table `alldata`   | Normalized, deduplicated, indexed clean data   |

---

## Database Schema

**Table name:** `alldata`

```sql
CREATE TABLE IF NOT EXISTS alldata (
    id          SERIAL PRIMARY KEY,
    platform    VARCHAR(50)    NOT NULL,
    title       TEXT           NOT NULL,
    price       NUMERIC(10,2)  DEFAULT 0,
    location    TEXT,
    rating      NUMERIC(4,2)   DEFAULT 0,
    url         TEXT UNIQUE,
    description TEXT,
    scraped_at  TIMESTAMP      NOT NULL DEFAULT NOW()
);
```

**Indexes:**

| Index Name             | Column     | Purpose                       |
|------------------------|------------|-------------------------------|
| `idx_alldata_price`    | `price`    | Fast price range queries       |
| `idx_alldata_location` | `location` | Fast location-based filtering  |
| `idx_alldata_platform` | `platform` | Fast platform filtering        |
| `idx_alldata_rating`   | `rating`   | Fast top-rated queries         |

---

## Verifying the Data

**View all stored listings:**
```bash
docker exec -it my-postgres psql -U ali -d mydb \
  -c "SELECT id, platform, title, price, location, rating FROM alldata ORDER BY id;"
```

**Count total records:**
```bash
docker exec -it my-postgres psql -U ali -d mydb \
  -c "SELECT COUNT(*) FROM alldata;"
```

**Top 5 most expensive listings:**
```bash
docker exec -it my-postgres psql -U ali -d mydb \
  -c "SELECT title, price, location FROM alldata ORDER BY price DESC LIMIT 5;"
```

**Top 5 highest rated listings:**
```bash
docker exec -it my-postgres psql -U ali -d mydb \
  -c "SELECT title, rating, location FROM alldata ORDER BY rating DESC LIMIT 5;"
```

**Listings grouped by location:**
```bash
docker exec -it my-postgres psql -U ali -d mydb \
  -c "SELECT location, COUNT(*) FROM alldata GROUP BY location ORDER BY COUNT(*) DESC;"
```

**View raw CSV output:**
```bash
cat output/raw_listings.csv
```

---
