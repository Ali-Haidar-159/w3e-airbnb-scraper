package airbnb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"airbnb-scraper/config"
	"airbnb-scraper/models"
	"airbnb-scraper/utils"

	"github.com/chromedp/chromedp"
)

// LocationSection represents a discovered location on the homepage
type LocationSection struct {
	Name string
	URL  string
}

// AirbnbScraper handles all Airbnb scraping operations
type AirbnbScraper struct {
	cfg         *config.Config
	logger      *utils.Logger
	rateLimiter *utils.RateLimiter
	mu          sync.Mutex
	seenURLs    map[string]bool
}

// NewAirbnbScraper creates a new AirbnbScraper
func NewAirbnbScraper(cfg *config.Config, logger *utils.Logger) *AirbnbScraper {
	return &AirbnbScraper{
		cfg:         cfg,
		logger:      logger,
		rateLimiter: utils.NewRateLimiter(cfg.RateLimitDelay),
		seenURLs:    make(map[string]bool),
	}
}

// newBrowserContext creates a fresh headless browser context
func (s *AirbnbScraper) newBrowserContext() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("log-level", "3"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1440, 900),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(func(string, ...interface{}) {}), // suppress noise
	)
	return ctx, func() {
		cancelCtx()
		cancelAlloc()
	}
}

// Scrape is the main entry point
func (s *AirbnbScraper) Scrape() ([]*models.RawListing, error) {
	s.logger.Info("Starting Airbnb scraper...")

	ctx, cancel := s.newBrowserContext()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Minute)
	defer cancelTimeout()

	// Step 1: discover sections from homepage
	sections, err := s.discoverSections(ctx)
	if err != nil || len(sections) == 0 {
		s.logger.Warn("Homepage JS discovery failed or returned 0 sections, using fallback search URLs...")
		sections = s.fallbackSections()
	}

	s.logger.Info("Scraping %d location sections...", len(sections))
	for i, sec := range sections {
		s.logger.Info("  [%d] %s", i+1, sec.Name)
	}

	// Step 2: scrape each section
	var allListings []*models.RawListing
	for _, section := range sections {
		s.rateLimiter.Wait()
		listings, err := s.scrapeSection(ctx, section)
		if err != nil {
			s.logger.Error("Section '%s' failed: %v", section.Name, err)
			continue
		}
		allListings = append(allListings, listings...)
		s.logger.Info("Section '%s' done: %d listings (total: %d)",
			section.Name, len(listings), len(allListings))
	}

	s.logger.Info("Scraping complete. Total raw listings: %d", len(allListings))
	return allListings, nil
}

// discoverSections tries to extract section links from the Airbnb homepage via JS
func (s *AirbnbScraper) discoverSections(ctx context.Context) ([]LocationSection, error) {
	s.logger.Info("Loading Airbnb homepage...")

	err := chromedp.Run(ctx,
		chromedp.Navigate(s.cfg.AirbnbURL),
		chromedp.Sleep(6*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("homepage load failed: %w", err)
	}

	s.logger.Info("Homepage loaded, extracting sections...")

	type sd struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	var raw []sd

	// Try multiple JS strategies to find section headings + their links
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var results = [];
			var seen = {};

			// Strategy 1: <section> with h2 + a[href*="/s/"]
			document.querySelectorAll('section').forEach(function(sec) {
				var h = sec.querySelector('h2');
				var a = sec.querySelector('a[href*="/s/"]');
				if (h && a && !seen[a.href]) {
					seen[a.href] = true;
					results.push({name: h.innerText.trim(), url: a.href});
				}
			});

			// Strategy 2: any h2 near an anchor with /s/
			if (results.length === 0) {
				document.querySelectorAll('h2').forEach(function(h2) {
					var name = h2.innerText.trim();
					if (!name) return;
					var p = h2.parentElement;
					for (var i = 0; i < 4; i++) {
						if (!p) break;
						var a = p.querySelector('a[href*="/s/"]');
						if (a && !seen[a.href]) {
							seen[a.href] = true;
							results.push({name: name, url: a.href});
							break;
						}
						p = p.parentElement;
					}
				});
			}

			// Strategy 3: all unique /s/ links with text
			if (results.length === 0) {
				document.querySelectorAll('a[href*="/s/"]').forEach(function(a) {
					var text = a.innerText.trim() || a.getAttribute('aria-label') || '';
					if (text && !seen[a.href]) {
						seen[a.href] = true;
						results.push({name: text, url: a.href});
					}
				});
			}

			return results;
		})()
	`, &raw))

	if err != nil {
		return nil, err
	}

	var sections []LocationSection
	for _, r := range raw {
		if r.Name == "" || r.URL == "" {
			continue
		}
		sections = append(sections, LocationSection{Name: r.Name, URL: r.URL})
	}
	return sections, nil
}

// fallbackSections returns hardcoded popular Airbnb search URLs
// used when homepage JS discovery fails due to Airbnb HTML changes
func (s *AirbnbScraper) fallbackSections() []LocationSection {
	base := "https://www.airbnb.com/s"
	return []LocationSection{
		{Name: "Bangkok", URL: base + "/Bangkok--Thailand/homes"},
		{Name: "Kuala Lumpur", URL: base + "/Kuala-Lumpur--Malaysia/homes"},
		{Name: "Tokyo", URL: base + "/Tokyo--Japan/homes"},
		{Name: "Bali", URL: base + "/Bali--Indonesia/homes"},
		{Name: "Seoul", URL: base + "/Seoul--South-Korea/homes"},
		{Name: "Singapore", URL: base + "/Singapore/homes"},
		{Name: "Paris", URL: base + "/Paris--France/homes"},
		{Name: "New York", URL: base + "/New-York--NY--United-States/homes"},
		{Name: "London", URL: base + "/London--United-Kingdom/homes"},
		{Name: "Dubai", URL: base + "/Dubai--United-Arab-Emirates/homes"},
	}
}

// scrapeSection collects PropertiesPerPage listings from a section, paginating as needed
func (s *AirbnbScraper) scrapeSection(ctx context.Context, section LocationSection) ([]*models.RawListing, error) {
	s.logger.Info("Scraping: %s", section.Name)

	var collected []*models.RawListing
	currentURL := section.URL
	page := 1

	for len(collected) < s.cfg.PropertiesPerPage {
		s.logger.Info("  [%s] page %d (have %d/%d)...",
			section.Name, page, len(collected), s.cfg.PropertiesPerPage)

		listings, nextURL, err := s.scrapePage(ctx, currentURL, section.Name)
		if err != nil {
			s.logger.Error("  Page %d error: %v", page, err)
			break
		}
		if len(listings) == 0 {
			s.logger.Warn("  No listings on page %d", page)
			break
		}

		for _, l := range listings {
			if len(collected) >= s.cfg.PropertiesPerPage {
				break
			}
			s.mu.Lock()
			if s.seenURLs[l.URL] {
				s.mu.Unlock()
				continue
			}
			s.seenURLs[l.URL] = true
			s.mu.Unlock()

			// Optionally enrich from detail page
			if l.Description == "" && l.URL != "" {
				s.rateLimiter.Wait()
				s.enrichDetail(ctx, l)
			}
			collected = append(collected, l)
		}

		if len(collected) >= s.cfg.PropertiesPerPage || nextURL == "" {
			break
		}
		currentURL = nextURL
		page++
		s.rateLimiter.Wait()
	}

	return collected, nil
}

// scrapePage navigates to a search result page and extracts all listing cards
func (s *AirbnbScraper) scrapePage(ctx context.Context, pageURL, sectionName string) ([]*models.RawListing, string, error) {
	var listings []*models.RawListing
	var nextURL string

	err := utils.RetryWithBackoff(s.cfg.MaxRetries, func() error {
		if err := chromedp.Run(ctx,
			chromedp.Navigate(pageURL),
			chromedp.Sleep(5*time.Second),
		); err != nil {
			return fmt.Errorf("navigate failed: %w", err)
		}

		// Try to wait for cards — but don't block if they don't appear
		_ = chromedp.Run(ctx, chromedp.WaitVisible(`[data-testid="card-container"]`, chromedp.ByQuery))

		type card struct {
			Title    string `json:"title"`
			Price    string `json:"price"`
			Rating   string `json:"rating"`
			URL      string `json:"url"`
			Location string `json:"location"`
		}
		var cards []card

		if err := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var results = [];

				// Find all listing containers using multiple selector strategies
				var containers = [];

				// Strategy A: official test id
				var a = document.querySelectorAll('[data-testid="card-container"]');
				if (a.length > 0) { containers = Array.from(a); }

				// Strategy B: itemprop
				if (containers.length === 0) {
					containers = Array.from(document.querySelectorAll('[itemprop="itemListElement"]'));
				}

				// Strategy C: parent div of any /rooms/ link
				if (containers.length === 0) {
					var seen = new Set();
					document.querySelectorAll('a[href*="/rooms/"]').forEach(function(a) {
						var p = a.parentElement;
						for (var i = 0; i < 5; i++) {
							if (!p) break;
							if (p.querySelectorAll('a[href*="/rooms/"]').length === 1) {
								if (!seen.has(p)) { seen.add(p); containers.push(p); }
								break;
							}
							p = p.parentElement;
						}
					});
				}

				containers.forEach(function(card) {
					// ── Title ──────────────────────────────────────────────
					var titleEl =
						card.querySelector('[data-testid="listing-card-title"]') ||
						card.querySelector('[id^="title_"]') ||
						card.querySelector('[itemprop="name"]');
					var title = titleEl ? titleEl.innerText.trim() : '';

					// ── Price ──────────────────────────────────────────────
					var price = '';
					// look for aria-label containing "per night"
					var priceAria = card.querySelector('[aria-label*="per night"]');
					if (priceAria) { price = priceAria.getAttribute('aria-label'); }
					// fallback: first span starting with $
					if (!price) {
						card.querySelectorAll('span').forEach(function(sp) {
							if (!price && sp.innerText.trim().startsWith('$')) {
								price = sp.innerText.trim();
							}
						});
					}

					// ── Rating ─────────────────────────────────────────────
					var rating = '';
					var ratingAria = card.querySelector('[aria-label*="out of 5"]');
					if (ratingAria) { rating = ratingAria.getAttribute('aria-label'); }
					if (!rating) {
						card.querySelectorAll('span').forEach(function(sp) {
							if (!rating && /^[345]\.\d{1,2}$/.test(sp.innerText.trim())) {
								rating = sp.innerText.trim();
							}
						});
					}

					// ── URL ────────────────────────────────────────────────
					var linkEl = card.querySelector('a[href*="/rooms/"]');
					var url = linkEl ? linkEl.href : '';

					// ── Location ───────────────────────────────────────────
					var location = '';
					if (title.includes(' in ')) {
						location = title.split(' in ').slice(1).join(' in ');
					}

					if (title || url) {
						results.push({title:title, price:price, rating:rating, url:url, location:location});
					}
				});

				return results;
			})()
		`, &cards)); err != nil {
			return fmt.Errorf("JS extraction failed: %w", err)
		}

		listings = nil
		for _, c := range cards {
			loc := c.Location
			if loc == "" {
				loc = sectionName
			}
			listings = append(listings, &models.RawListing{
				Platform:  "Airbnb",
				Title:     c.Title,
				RawPrice:  c.Price,
				Location:  loc,
				RawRating: c.Rating,
				URL:       c.URL,
				ScrapedAt: time.Now(),
			})
		}

		// Next page link
		var next string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var n = document.querySelector('a[aria-label="Next"]') ||
				        document.querySelector('[data-testid="pagination-next-btn"]') ||
				        document.querySelector('a[href*="items_offset"]');
				return n ? n.href : '';
			})()
		`, &next))
		nextURL = next
		return nil
	}, s.logger)

	return listings, nextURL, err
}

// enrichDetail fetches the listing detail page to grab description
func (s *AirbnbScraper) enrichDetail(ctx context.Context, listing *models.RawListing) {
	if listing.URL == "" {
		return
	}
	s.logger.Debug("  Enriching: %s", listing.Title)

	var desc string
	err := chromedp.Run(ctx,
		chromedp.Navigate(listing.URL),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
			(function() {
				var el =
					document.querySelector('[data-section-id="DESCRIPTION_DEFAULT"] span') ||
					document.querySelector('[data-section-id="OVERVIEW_DEFAULT"] h1') ||
					document.querySelector('h1');
				return el ? el.innerText.trim() : '';
			})()
		`, &desc),
	)
	if err != nil {
		s.logger.Warn("  Enrich failed for '%s': %v", listing.Title, err)
		return
	}
	if desc != "" && !strings.EqualFold(strings.TrimSpace(desc), strings.TrimSpace(listing.Title)) {
		listing.Description = desc
	}
}
