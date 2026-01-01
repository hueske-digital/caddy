#!/bin/bash
set -e

# Usage: ./integration.sh [--keep-stack]
#   --keep-stack  Don't stop the caddy stack after tests (useful for debugging)

KEEP_STACK=false
for arg in "$@"; do
    case $arg in
        --keep-stack) KEEP_STACK=true ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test directory (relative to script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"
TEST_DIR="/tmp/watcher-integration-test-$$"
COMPOSE_PROJECT="watchertest"

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

run_test() {
    ((TESTS_RUN++))
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Test $TESTS_RUN: $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

cleanup() {
    log_info "Cleaning up..."

    # Stop test app if running
    if [ -d "$TEST_DIR" ]; then
        cd "$TEST_DIR"
        docker compose -p "$COMPOSE_PROJECT" down --volumes --remove-orphans 2>/dev/null || true
    fi

    # Remove test directory
    rm -rf "$TEST_DIR"

    # Stop caddy stack if we started it (unless --keep-stack)
    if [ "$STARTED_STACK" = true ] && [ "$KEEP_STACK" = false ]; then
        log_info "Stopping caddy stack (we started it)..."
        cd "$PROJECT_DIR"
        docker compose down
    elif [ "$STARTED_STACK" = true ] && [ "$KEEP_STACK" = true ]; then
        log_info "Keeping caddy stack running (--keep-stack)"
    fi

    log_info "Cleanup complete"
}

# Trap to cleanup on exit
trap cleanup EXIT

# Track if we started the stack
STARTED_STACK=false

# Setup
setup() {
    log_info "Setting up test environment..."
    mkdir -p "$TEST_DIR/hosts/internal" "$TEST_DIR/hosts/external" "$TEST_DIR/hosts/cloudflare"

    # Check if caddy watcher is running, if not start it
    if ! docker ps --format '{{.Names}}' | grep -q "caddy.*watcher\|proxy.*watcher"; then
        log_info "Caddy stack not running, starting it..."
        cd "$PROJECT_DIR"
        docker compose up -d --wait
        STARTED_STACK=true
        sleep 3  # Give watcher time to initialize
        log_info "Caddy stack started"
    else
        log_info "Caddy stack already running"
    fi

    log_info "Test environment ready at $TEST_DIR"
}

# Wait for condition with timeout
wait_for() {
    local condition="$1"
    local timeout="${2:-10}"
    local interval="${3:-1}"
    local elapsed=0

    while ! eval "$condition"; do
        sleep "$interval"
        elapsed=$((elapsed + interval))
        if [ "$elapsed" -ge "$timeout" ]; then
            return 1
        fi
    done
    return 0
}

# ============================================================================
# TEST CASES
# ============================================================================

test_service_start() {
    run_test "Service start creates config"

    cd "$TEST_DIR"

    # Create test compose file
    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-start.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    # Start service
    docker compose -p "$COMPOSE_PROJECT" up -d

    # Wait for config to appear in caddy's hosts directory
    HOSTS_DIR="$PROJECT_DIR/hosts"
    if wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10; then
        log_pass "Config file created"
    else
        log_fail "Config file not created within timeout"
        docker compose -p "$COMPOSE_PROJECT" logs
        return 1
    fi

    # Check config content
    if grep -q "test-start.example.com" "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"; then
        log_pass "Config contains correct domain"
    else
        log_fail "Config does not contain correct domain"
        cat "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_service_stop() {
    run_test "Service stop keeps config"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-stop.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    # Stop (not down) the service
    docker compose -p "$COMPOSE_PROJECT" stop
    sleep 2

    # Config should still exist
    if [ -f "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf" ]; then
        log_pass "Config file preserved after stop"
    else
        log_fail "Config file was removed after stop"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_service_down() {
    run_test "Service down removes config and network"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-down.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    # Down the service
    docker compose -p "$COMPOSE_PROJECT" down
    sleep 2

    # Config should be removed
    if [ ! -f "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf" ]; then
        log_pass "Config file removed after down"
    else
        log_fail "Config file was NOT removed after down"
    fi

    # Network should be removed (no "in use" error)
    if ! docker network ls --format '{{.Name}}' | grep -q "${COMPOSE_PROJECT}_caddy"; then
        log_pass "Network removed without error"
    else
        log_fail "Network still exists"
    fi
}

test_external_type() {
    run_test "External type config"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-external.example.com
      - CADDY_TYPE=external
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"

    if wait_for "[ -f '$HOSTS_DIR/external/${COMPOSE_PROJECT}_caddy.conf' ]" 10; then
        log_pass "External config created in correct directory"
    else
        log_fail "External config not created"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_cloudflare_type() {
    run_test "Cloudflare type config"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-cloudflare.example.com
      - CADDY_TYPE=cloudflare
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"

    if wait_for "[ -f '$HOSTS_DIR/cloudflare/${COMPOSE_PROJECT}_caddy.conf' ]" 10; then
        log_pass "Cloudflare config created in correct directory"

        if grep -q "import cloudflare" "$HOSTS_DIR/cloudflare/${COMPOSE_PROJECT}_caddy.conf"; then
            log_pass "Config contains import cloudflare"
        else
            log_fail "Config missing import cloudflare"
        fi
    else
        log_fail "Cloudflare config not created"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_logging_option() {
    run_test "Logging option"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-logging.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
      - CADDY_LOGGING=true
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    if grep -q "import logging" "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"; then
        log_pass "Config contains import logging"
    else
        log_fail "Config missing import logging"
        cat "$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_disabled_options() {
    run_test "Disabled options (TLS, compression, header)"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-disabled.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
      - CADDY_TLS=false
      - CADDY_COMPRESSION=false
      - CADDY_HEADER=false
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    CONFIG_FILE="$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"

    if ! grep -q "import tls" "$CONFIG_FILE"; then
        log_pass "Config does NOT contain import tls (disabled)"
    else
        log_fail "Config contains import tls but should not"
    fi

    if ! grep -q "import compression" "$CONFIG_FILE"; then
        log_pass "Config does NOT contain import compression (disabled)"
    else
        log_fail "Config contains import compression but should not"
    fi

    if ! grep -q "import header" "$CONFIG_FILE"; then
        log_pass "Config does NOT contain import header (disabled)"
    else
        log_fail "Config contains import header but should not"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_multiple_domains() {
    run_test "Multiple domains"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=a.example.com, b.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    CONFIG_FILE="$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"

    if grep -q "a.example.com" "$CONFIG_FILE" && grep -q "b.example.com" "$CONFIG_FILE"; then
        log_pass "Config contains both domains"
    else
        log_fail "Config missing domains"
        cat "$CONFIG_FILE"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_file_ownership() {
    run_test "File ownership is 1000:1000"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-owner.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    HOSTS_DIR="$PROJECT_DIR/hosts"
    wait_for "[ -f '$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf' ]" 10

    CONFIG_FILE="$HOSTS_DIR/internal/${COMPOSE_PROJECT}_caddy.conf"
    OWNER=$(stat -f '%u:%g' "$CONFIG_FILE" 2>/dev/null || stat -c '%u:%g' "$CONFIG_FILE" 2>/dev/null)

    if [ "$OWNER" = "1000:1000" ]; then
        log_pass "File owned by 1000:1000"
    else
        log_fail "File owned by $OWNER (expected 1000:1000)"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

test_status_api() {
    run_test "Status API returns JSON"

    cd "$TEST_DIR"

    cat > docker-compose.yml << 'EOF'
services:
  testapp:
    image: nginx:alpine
    environment:
      - CADDY_DOMAIN=test-api.example.com
      - CADDY_TYPE=internal
      - CADDY_PORT=80
    networks:
      - caddy

networks:
  caddy:
EOF

    docker compose -p "$COMPOSE_PROJECT" up -d
    sleep 3

    # Find the watcher container and exec curl
    WATCHER=$(docker ps --format '{{.Names}}' | grep -E "caddy.*watcher|proxy.*watcher" | head -1)

    if [ -n "$WATCHER" ]; then
        # Check if status API is reachable from inside the container
        RESPONSE=$(docker exec "$WATCHER" wget -qO- http://localhost:8080/api/status 2>/dev/null || echo "failed")

        if echo "$RESPONSE" | grep -q '"services"'; then
            log_pass "Status API returns valid JSON"
        else
            log_fail "Status API response invalid"
            echo "$RESPONSE"
        fi
    else
        log_fail "Could not find watcher container"
    fi

    docker compose -p "$COMPOSE_PROJECT" down
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════╗"
    echo "║           Caddy Watcher Integration Tests                        ║"
    echo "╚══════════════════════════════════════════════════════════════════╝"
    echo ""

    setup

    # Run all tests
    test_service_start
    test_service_stop
    test_service_down
    test_external_type
    test_cloudflare_type
    test_logging_option
    test_disabled_options
    test_multiple_domains
    test_file_ownership
    test_status_api

    # Summary
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════╗"
    echo "║                         TEST SUMMARY                             ║"
    echo "╚══════════════════════════════════════════════════════════════════╝"
    echo ""
    echo -e "Tests run:    $TESTS_RUN"
    echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
    echo ""

    if [ "$TESTS_FAILED" -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

main "$@"
