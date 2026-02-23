#!/bin/bash

# ============================================================
#  Airbnb Rental Market Scraping System — Auto Setup & Run
# ============================================================

set -e  # Exit immediately if any command fails

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "\n${BLUE}========== $1 ==========${NC}"; }

# ─── Config ────────────────────────────────────────────────
DB_CONTAINER_NAME="my-postgres"
DB_USER="ali"
DB_PASSWORD="1234"
DB_NAME="mydb"
DB_PORT="5432"
DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"

# ─── Step 1: Check prerequisites ───────────────────────────
log_step "Checking Prerequisites"

# Check Go
if ! command -v go &> /dev/null; then
    log_error "Go is not installed. Please install Go 1.21+ from https://go.dev/dl/"
    exit 1
fi
log_info "Go found: $(go version)"

# Check Docker
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed. Please install Docker."
    exit 1
fi
log_info "Docker found: $(docker --version)"

# Check Chromium
CHROMIUM_BIN=""
for bin in chromium-browser chromium google-chrome google-chrome-stable; do
    if command -v "$bin" &> /dev/null; then
        CHROMIUM_BIN="$bin"
        break
    fi
done

if [ -z "$CHROMIUM_BIN" ]; then
    log_error "Chromium is not installed. Please run: sudo apt install -y chromium-browser"
    exit 1
fi
log_info "Chromium found: $($CHROMIUM_BIN --version)"

# ─── Step 2: Start PostgreSQL Docker container ─────────────
log_step "Setting Up PostgreSQL"

# Check if container already exists
if docker ps -a --format '{{.Names}}' | grep -q "^${DB_CONTAINER_NAME}$"; then
    # Container exists — check if it's running
    if docker ps --format '{{.Names}}' | grep -q "^${DB_CONTAINER_NAME}$"; then
        log_info "PostgreSQL container '${DB_CONTAINER_NAME}' is already running."
    else
        log_info "Starting existing PostgreSQL container '${DB_CONTAINER_NAME}'..."
        docker start "${DB_CONTAINER_NAME}"
    fi
else
    # Container doesn't exist — create it
    log_info "Creating PostgreSQL Docker container '${DB_CONTAINER_NAME}'..."
    docker run --name "${DB_CONTAINER_NAME}" \
        -e POSTGRES_USER="${DB_USER}" \
        -e POSTGRES_PASSWORD="${DB_PASSWORD}" \
        -e POSTGRES_DB="${DB_NAME}" \
        -p "${DB_PORT}":5432 \
        -v pgdata:/var/lib/postgresql/data \
        -d postgres:16
fi

# Wait for PostgreSQL to be ready
log_info "Waiting for PostgreSQL to be ready..."
for i in $(seq 1 15); do
    if docker exec "${DB_CONTAINER_NAME}" pg_isready -U "${DB_USER}" -d "${DB_NAME}" &> /dev/null; then
        log_info "PostgreSQL is ready."
        break
    fi
    if [ "$i" -eq 15 ]; then
        log_error "PostgreSQL did not become ready in time. Check Docker."
        exit 1
    fi
    sleep 2
done

# ─── Step 3: Install Go dependencies ───────────────────────
log_step "Installing Go Dependencies"

if [ ! -f "go.mod" ]; then
    log_error "go.mod not found. Make sure you are running this script from the project root directory."
    exit 1
fi

# Remove stale go.sum if it causes version mismatch
if [ -f "go.sum" ]; then
    log_info "go.sum found, running go mod tidy to verify..."
else
    log_info "No go.sum found, downloading dependencies..."
fi

go mod tidy
log_info "Dependencies ready."

# ─── Step 4: Create output directory ───────────────────────
log_step "Preparing Output Directory"
mkdir -p output
log_info "Output directory ready."

# ─── Step 5: Build the project ─────────────────────────────
log_step "Building Project"
go build -o airbnb-scraper-bin .
log_info "Build successful."

# ─── Step 6: Run the scraper ───────────────────────────────
log_step "Running Airbnb Scraper"
log_info "This may take 3–8 minutes. Please wait..."
echo ""

DATABASE_URL="${DATABASE_URL}" \
PROPERTIES_PER_SECTION=10 \
RATE_LIMIT_DELAY_MS=2000 \
MAX_RETRIES=3 \
MAX_CONCURRENCY=3 \
CSV_FILE_PATH="output/raw_listings.csv" \
./airbnb-scraper-bin

# ─── Step 7: Verify output ─────────────────────────────────
log_step "Verifying Output"

# Check CSV
if [ -f "output/raw_listings.csv" ]; then
    ROW_COUNT=$(( $(wc -l < output/raw_listings.csv) - 1 ))
    log_info "CSV file created: output/raw_listings.csv (${ROW_COUNT} listings)"
else
    log_warn "CSV file not found. Scraper may have had issues."
fi

# Check DB
log_info "Checking PostgreSQL records..."
DB_COUNT=$(docker exec "${DB_CONTAINER_NAME}" \
    psql -U "${DB_USER}" -d "${DB_NAME}" -t -c "SELECT COUNT(*) FROM alldata;" 2>/dev/null | tr -d ' ')

if [ -n "$DB_COUNT" ] && [ "$DB_COUNT" -gt 0 ]; then
    log_info "PostgreSQL table 'alldata' contains ${DB_COUNT} records."
else
    log_warn "No records found in PostgreSQL. Check scraper logs above."
fi

# ─── Done ──────────────────────────────────────────────────
echo ""
echo -e "${GREEN}============================================================${NC}"
echo -e "${GREEN}  All done!${NC}"
echo -e "${GREEN}  Raw CSV   : output/raw_listings.csv${NC}"
echo -e "${GREEN}  Database  : PostgreSQL table 'alldata' (${DB_COUNT} rows)${NC}"
echo -e "${GREEN}============================================================${NC}"
echo ""
echo "To manually inspect the database:"
echo "  docker exec -it ${DB_CONTAINER_NAME} psql -U ${DB_USER} -d ${DB_NAME} -c \"SELECT title, price, location, rating FROM alldata LIMIT 10;\""
