#!/bin/sh
# verify-proxy.sh — run on the production Linux VM after nginx cutover
# Verifies that:
#   1. ClientIP seen by Go matches the real peer (not spoofed X-Forwarded-For)
#   2. Per-IP rate limits are independent (one IP gets 429, another does not)
#
# Requires: curl, jq, and a valid session cookie (run after logging in)
# Usage: ./scripts/verify-proxy.sh <DOMAIN> <SESSION_COOKIE_FILE>

set -eu

if [ $# -ne 2 ]; then
  echo "Usage: $0 <DOMAIN> <SESSION_COOKIE_FILE>"
  echo "Example: $0 goodspot.example.com /tmp/cookies.txt"
  exit 1
fi

DOMAIN="$1"
COOKIE_FILE="$2"
API="https://${DOMAIN}"

# 1. Verify spoofed X-Forwarded-For is ignored
echo "=== Test 1: Spoofed X-Forwarded-For ==="
echo "Sending request with X-Forwarded-For: 1.2.3.4"
code=$(curl -sk --resolve "${DOMAIN}:443:127.0.0.1" \
  -H 'X-Forwarded-For: 1.2.3.4' \
  -b "${COOKIE_FILE}" \
  "${API}/api/categories" -o /dev/null -w "%{http_code}")
echo "HTTP ${code}"

# Check Go logs for the ClientIP (run this on the VM where docker logs are available)
echo ""
echo "Check the Go server logs for the ClientIP seen:"
echo "  docker compose -f docker-compose.prod.yml logs --since 5s server | grep '\"ip\"' | tail -1"
echo "Expected: the real client IP (what nginx saw as \$remote_addr), NOT 1.2.3.4"
echo "and NOT a 10.89.0.x container address (that would mean per-IP buckets are shared again)."
echo ""

# 2. Verify per-IP rate limits are independent
echo "=== Test 2: Per-IP rate limit independence ==="
echo "This test requires two different source IPs."
echo "On a Linux VM with multiple interfaces or using network namespaces,"
echo "run 11 PATCH requests from IP A and 11 from IP B."
echo "IP A should get 429 on the 11th; IP B should still succeed."

# Helper: run N requests from a specific source IP (requires root for --interface)
run_patches() {
  local ip="$1"
  local pin_id="$2"
  local cookie="$3"
  local count=0
  local last_code=0

  for i in $(seq 1 11); do
    if [ -n "$ip" ]; then
      code=$(curl -sk --resolve "${DOMAIN}:443:127.0.0.1" \
        --interface "$ip" \
        -b "$cookie" \
        -X PATCH "${API}/api/pins/${pin_id}" \
        -H 'Content-Type: application/json' -d '{}' \
        -o /dev/null -w "%{http_code}")
    else
      code=$(curl -sk --resolve "${DOMAIN}:443:127.0.0.1" \
        -b "$cookie" \
        -X PATCH "${API}/api/pins/${pin_id}" \
        -H 'Content-Type: application/json' -d '{}' \
        -o /dev/null -w "%{http_code}")
    fi
    echo "  Request $i from ${ip:-default}: HTTP $code"
    last_code=$code
  done

  if [ "$last_code" = "429" ]; then
    echo "  -> Rate limited as expected (429 on 11th request)"
  else
    echo "  -> ERROR: Expected 429 on 11th request, got $last_code"
  fi
}

# This is a template - the actual IP addresses must be configured for your VM
echo ""
echo "To run the rate limit test from two IPs:"
echo "  1. Find two source IPs on your VM (e.g., eth0 and eth1, or create veth pairs)"
echo "  2. Create a test pin and note its ID"
echo "  3. Run: run_patches <IP_A> <PIN_ID> <COOKIE_FILE>"
echo "  4. Run: run_patches <IP_B> <PIN_ID> <COOKIE_FILE>"
echo "  5. Verify IP_A gets 429 on request 11, IP_B does NOT"

echo ""
echo "=== Test 3: Certificate serial check ==="
echo "Current certificate serial:"
openssl s_client -connect 127.0.0.1:443 -servername "${DOMAIN}" </dev/null 2>/dev/null | openssl x509 -noout -serial

echo ""
echo "=== Test 4: Security headers ==="
for path in "/" "/api/categories" "/events"; do
  echo "Checking $path..."
  curl -skI --resolve "${DOMAIN}:443:127.0.0.1" "${API}${path}" | grep -iE "Strict-Transport-Security|X-Content-Type-Options|X-Frame-Options|Referrer-Policy|Server:"
done

echo ""
echo "All manual checks complete. Review output above."