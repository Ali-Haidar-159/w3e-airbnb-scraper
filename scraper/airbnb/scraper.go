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

// newContext creates a fresh chromedp context (one browser, one tab at a time)
func (s *AirbnbScraper) newContext() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("log-level", "3"), // suppress Chrome logs
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1280, 900),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...interface{}) {}))

	cancel := func() {
		cancelCtx()
		cancelAlloc()
	}
	return ctx, cancel
}

// Scrape is the main entry point
func (s *AirbnbScraper) Scrape() ([]*models.RawListing, error) {
	s.logger.Info("Starting Airbnb scraper...")

	ctx, cancel := s.newContext()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 25*time.Minute)
	defer cancelTimeout()

	// Step 1: Discover location sections from homepage
	sections, err := s.discoverSections(ctx)
	if err != nil {
		return nil, fmt.Errorf("section discovery failed: %w", err)
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found on Airbnb homepage")
	}
	s.logger.Info("Found %d location sections", len(sections))
	for i, sec := range sections {
		s.logger.Info("  [%d] %s -> %s", i+1, sec.Name, sec.URL)
	}

	// Step 2: Scrape each section sequentially (avoid overwhelming Airbnb)
	var allListings []*models.RawListing
	for _, section := range sections {
		s.rateLimiter.Wait()
		listings, err := s.scrapeSection(ctx, section)
		if err != nil {
			s.logger.Error("Section '%s' failed: %v", section.Name, err)
			continue
		}
		allListings = append(allListings, listings...)
		s.logger.Info("Section '%s': collected %d listings (total so far: %d)",
			section.Name, len(listings), len(allListings))
	}

	s.logger.Info("Scraping complete. Total raw listings: %d", len(allListings))
	return allListings, nil
}

// discoverSections loads the homepage and finds all "X in City" section links
func (s *AirbnbScraper) discoverSections(ctx context.Context) ([]LocationSection, error) {
	s.logger.Info("Loading Airbnb homepage...")

	// Navigate with a simple load wait, not waiting for specific element
	err := chromedp.Run(ctx,
		chromedp.Navigate(s.cfg.AirbnbURL),
		chromedp.Sleep(5*time.Second), // give JS time to render
	)
	if err != nil {
		return nil, fmt.Errorf("homepage navigation failed: %w", err)
	}

	s.logger.Info("Homepage loaded, extracting sections...")

	// Extract all section heading + link pairs via JS
	type sectionData struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	var rawSections []sectionData

	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(function() {
			var results = [];
			var seen = {};

			// Strategy 1: find <section> elements with an <a href="/s/...">
			document.querySelectorAll('section').forEach(function(sec) {
				var heading = sec.querySelector('h2');
				var link = sec.querySelector('a[href*="/s/"]');
				if (!heading || !link) return;
				var name = heading.innerText.trim();
				var href = link.href;
				if (name && href && !seen[href]) {
					seen[href] = true;
					results.push({name: name, url: href});
				}
			});

			// Strategy 2: find heading > sibling/child links (Airbnb sometimes uses div-based sections)
			if (results.length === 0) {
				document.querySelectorAll('h2').forEach(function(h2) {
					var name = h2.innerText.trim();
					if (!name) return;
					// Look for a nearby anchor
					var parent = h2.closest('div[class]') || h2.parentElement;
					if (!parent) return;
					var link = parent.querySelector('a[href*="/s/"]');
					if (!link) {
						// try going up one more level
						parent = parent.parentElement;
						if (parent) link = parent.querySelector('a[href*="/s/"]');
					}
					if (link && !seen[link.href]) {
						seen[link.href] = true;
						results.push({name: name, url: link.href});
					}
				});
			}

			return results;
		})()
	`, &rawSections))

	if err != nil {
		return nil, fmt.Errorf("JS section extraction failed: %w", err)
	}

	var sections []LocationSection
	for _, rs := range rawSections {
		if rs.Name == "" || rs.URL == "" {
			continue
		}
		sections = append(sections, LocationSection{Name: rs.Name, URL: rs.URL})
	}

	return sections, nil
}

// scrapeSection scrapes PropertiesPerPage listings from a section, paginating as needed
func (s *AirbnbScraper) scrapeSection(ctx context.Context, section LocationSection) ([]*models.RawListing, error) {
	s.logger.Info("Scraping section: %s", section.Name)

	var allListings []*models.RawListing
	currentURL := section.URL
	page := 1

	for len(allListings) < s.cfg.PropertiesPerPage {
		s.logger.Info("  [%s] page %d (have %d/%d)...", section.Name, page, len(allListings), s.cfg.PropertiesPerPage)

		listings, nextURL, err := s.scrapePage(ctx, currentURL, section.Name)
		if err != nil {
			s.logger.Error("  Page %d error: %v", page, err)
			break
		}
		if len(listings) == 0 {
			s.logger.Warn("  No listings found on page %d", page)
			break
		}

		for _, l := range listings {
			if len(allListings) >= s.cfg.PropertiesPerPage {
				break
			}
			// Dedup by URL
			s.mu.Lock()
			if s.seenURLs[l.URL] {
				s.mu.Unlock()
				continue
			}
			s.seenURLs[l.URL] = true
			s.mu.Unlock()

			// Enrich if needed
			if l.Description == "" && l.URL != "" {
				s.rateLimiter.Wait()
				s.enrichDetail(ctx, l)
			}
			allListings = append(allListings, l)
		}

		if nextURL == "" || len(allListings) >= s.cfg.PropertiesPerPage {
			break
		}
		currentURL = nextURL
		page++
		s.rateLimiter.Wait()
	}

	return allListings, nil
}

// scrapePage navigates to a search results page and extracts listing cards
func (s *AirbnbScraper) scrapePage(ctx context.Context, pageURL string, sectionName string) ([]*models.RawListing, string, error) {
	var listings []*models.RawListing
	var nextURL string

	err := utils.RetryWithBackoff(s.cfg.MaxRetries, func() error {
		// Navigate
		err := chromedp.Run(ctx,
			chromedp.Navigate(pageURL),
			chromedp.Sleep(4*time.Second), // wait for JS render
		)
		if err != nil {
			return fmt.Errorf("navigate failed: %w", err)
		}

		// Try to wait for any listing card (try multiple selectors)
		waitErr := chromedp.Run(ctx, chromedp.WaitVisible(`[itemprop="itemListElement"]`, chromedp.ByQuery))
		if waitErr != nil {
			// fallback: just wait a bit more
			_ = chromedp.Run(ctx, chromedp.Sleep(3*time.Second))
		}

		// Extract cards
		type cardData struct {
			Title    string `json:"title"`
			Price    string `json:"price"`
			Rating   string `json:"rating"`
			URL      string `json:"url"`
			Location string `json:"location"`
		}
		var cards []cardData

		jsErr := chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var cards = [];

				// Approach A: data-testid="card-container"
				var containers = document.querySelectorAll('[data-testid="card-container"]');

				// Approach B: itemprop listing items
				if (containers.length === 0) {
					containers = document.querySelectorAll('[itemprop="itemListElement"]');
				}

				// Approach C: any div with a room link child
				if (containers.length === 0) {
					var links = document.querySelectorAll('a[href*="/rooms/"]');
					var parents = new Set();
					links.forEach(function(a) {
						var p = a.closest('div[class]');
						if (p) parents.add(p);
					});
					containers = Array.from(parents);
				}

				containers.forEach(function(card) {
					// Title
					var titleEl = card.querySelector('[data-testid="listing-card-title"]') ||
					              card.querySelector('[id^="title_"]') ||
					              card.querySelector('[itemprop="name"]') ||
					              card.querySelector('div[class*="t1ms2xzu"]');
					var title = titleEl ? titleEl.innerText.trim() : '';

					// Price: look for $ in any span
					var price = '';
					var spans = card.querySelectorAll('span');
					for (var i = 0; i < spans.length; i++) {
						var t = spans[i].innerText.trim();
						if (t.startsWith('$') && t.length < 40) {
							price = t;
							break;
						}
					}
					// Also check aria-label on price containers
					if (!price) {
						var priceEl = card.querySelector('[aria-label*="per night"]') ||
						              card.querySelector('[class*="price"]');
						if (priceEl) price = priceEl.innerText.trim();
					}

					// Rating: look for pattern like "4.xx"
					var rating = '';
					for (var j = 0; j < spans.length; j++) {
						var rt = spans[j].innerText.trim();
						if (/^[345]\.\d{1,2}$/.test(rt)) {
							rating = rt;
							break;
						}
					}
					// Also check aria-label="X out of 5"
					if (!rating) {
						var ratingEl = card.querySelector('[aria-label*="out of 5"]');
						if (ratingEl) rating = ratingEl.getAttribute('aria-label') || '';
					}

					// URL
					var linkEl = card.querySelector('a[href*="/rooms/"]');
					var url = linkEl ? linkEl.href : '';

					// Location from title
					var location = '';
					if (title.includes(' in ')) {
						location = title.split(' in ').slice(1).join(' in ');
					}

					if (title || url) {
						cards.push({title:title, price:price, rating:rating, url:url, location:location});
					}
				});

				return cards;
			})()
		`, &cards))
		if jsErr != nil {
			return fmt.Errorf("card JS failed: %w", jsErr)
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

		// Find next page link
		var next string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`
			(function() {
				var btn = document.querySelector('a[aria-label="Next"]') ||
				          document.querySelector('[data-testid="pagination-next-btn"]') ||
				          document.querySelector('a[href*="items_offset"]') ||
				          document.querySelector('a[href*="pagination"]');
				return btn ? btn.href : '';
			})()
		`, &next))
		nextURL = next
		return nil
	}, s.logger)

	return listings, nextURL, err
}

// enrichDetail visits a listing's detail page to grab the description
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
				var el = document.querySelector('[data-section-id="DESCRIPTION_DEFAULT"] span') ||
				         document.querySelector('[data-section-id="OVERVIEW_DEFAULT"] h1') ||
				         document.querySelector('h1[class*="hpipapi"]') ||
				         document.querySelector('h1');
				return el ? el.innerText.trim() : '';
			})()
		`, &desc),
	)
	if err != nil {
		s.logger.Warn("  Enrich failed for '%s': %v", listing.Title, err)
		return
	}
	// If desc is just repeating the title, skip
	if desc != "" && !strings.EqualFold(strings.TrimSpace(desc), strings.TrimSpace(listing.Title)) {
		listing.Description = desc
	}
}
