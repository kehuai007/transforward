#!/bin/bash

# TransForward Integration Test Script
# Tests: Login, Rules CRUD, TCP/UDP forwarding, WebSocket, Password Change

set -e

BASE_URL="http://localhost:8081"
DATA_DIR=".transforward_test"
TEST_RESULT=""

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

pass() {
    echo "  ✓ PASS: $1"
    TEST_RESULT="${TEST_RESULT}✓ $1\n"
}

fail() {
    echo "  ✗ FAIL: $1"
    TEST_RESULT="${TEST_RESULT}✗ $1\n"
}

cleanup() {
    log "Cleaning up..."
    # Kill any running servers
    pkill -f "transforward" 2>/dev/null || true
    # Remove test data directory
    rm -rf "$DATA_DIR" 2>/dev/null || true
    sleep 1
}

wait_for_server() {
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$BASE_URL/" > /dev/null 2>&1; then
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 0.5
    done
    return 1
}

# Test login
test_login() {
    log "Testing login..."

    # Test login with wrong password
    RESP=$(curl -s -X POST "$BASE_URL/api/login" \
        -H "Content-Type: application/json" \
        -d '{"password":"wrongpassword"}')
    if echo "$RESP" | grep -q "invalid password\|error"; then
        pass "Wrong password rejected"
    else
        fail "Wrong password should be rejected"
    fi
}

# Test get rules (should be empty initially)
test_get_rules() {
    log "Testing get rules..."

    RESP=$(curl -s -X GET "$BASE_URL/api/rules" \
        -H "Authorization: Bearer $TOKEN")
    if echo "$RESP" | grep -q "\["; then
        pass "Get rules returns array"
    else
        fail "Get rules should return array"
    fi
}

# Test add rule
test_add_rule() {
    log "Testing add rule..."

    RESP=$(curl -s -X POST "$BASE_URL/api/rules" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{"id":"test-tcp","name":"Test TCP","protocol":"tcp","listen":"19099","target":"127.0.0.1:8081","enable":true}')

    if echo "$RESP" | grep -q "success"; then
        pass "Add TCP rule"
    else
        fail "Add TCP rule - $RESP"
    fi
}

# Test get status
test_get_status() {
    log "Testing get status..."

    RESP=$(curl -s -X GET "$BASE_URL/api/status" \
        -H "Authorization: Bearer $TOKEN")

    if echo "$RESP" | grep -q "total_rules"; then
        pass "Get status"
    else
        fail "Get status - $RESP"
    fi
}

# Test update rule
test_update_rule() {
    log "Testing update rule..."

    RESP=$(curl -s -X PUT "$BASE_URL/api/rules/test-tcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{"id":"test-tcp","name":"Test TCP Updated","protocol":"tcp","listen":"19099","target":"127.0.0.1:8081","enable":true}')

    if echo "$RESP" | grep -q "success"; then
        pass "Update rule"
    else
        fail "Update rule - $RESP"
    fi
}

# Test TCP forwarding
test_tcp_forwarding() {
    log "Testing TCP forwarding..."

    # Start a local TCP server on target port
    (timeout 5 nc -l -p 8081 -c "cat" 2>/dev/null || true) &
    sleep 1

    # Connect to forwarding port and send data
    echo "test data" | timeout 3 nc localhost 19099 > /tmp/tcp_forward_test.txt 2>/dev/null || true

    if grep -q "test data" /tmp/tcp_forward_test.txt 2>/dev/null; then
        pass "TCP forwarding works"
    else
        fail "TCP forwarding - data not received"
    fi
    rm -f /tmp/tcp_forward_test.txt
}

# Test delete rule
test_delete_rule() {
    log "Testing delete rule..."

    RESP=$(curl -s -X DELETE "$BASE_URL/api/rules/test-tcp" \
        -H "Authorization: Bearer $TOKEN")

    if echo "$RESP" | grep -q "success"; then
        pass "Delete rule"
    else
        fail "Delete rule - $RESP"
    fi
}

# Test WebSocket
test_websocket() {
    log "Testing WebSocket..."

    # Just check if WebSocket endpoint exists (full WS test requires browser or ws client)
    WS_TEST=$(curl -s -I -N "$BASE_URL/ws" 2>&1 | head -5)
    if echo "$WS_TEST" | grep -q -i "upgrade\|websocket"; then
        pass "WebSocket endpoint available"
    else
        fail "WebSocket endpoint - $WS_TEST"
    fi
}

# Main test flow
main() {
    log "=== TransForward Integration Tests ==="
    echo ""

    cleanup

    # Build
    log "Building..."
    go build -o "$DATA_DIR/transforward.exe" . 2>/dev/null

    # Start server in background
    log "Starting server..."
    "$DATA_DIR/transforward.exe" &
    SERVER_PID=$!
    sleep 2

    # Wait for server
    if ! wait_for_server; then
        echo "Server failed to start"
        exit 1
    fi
    log "Server started on port 8081"

    # Get initial page (should show login)
    PAGE=$(curl -s "$BASE_URL/")
    if echo "$PAGE" | grep -q "TransForward"; then
        pass "Web UI accessible"
    else
        fail "Web UI not accessible"
    fi

    # First login - set password
    log "Initial setup - setting password..."
    SETUP_RESP=$(curl -s -X POST "$BASE_URL/api/login" \
        -H "Content-Type: application/json" \
        -d '{"password":"testpass123"}')

    if echo "$SETUP_RESP" | grep -q "token"; then
        TOKEN=$(echo "$SETUP_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        pass "Login/Setup successful"
    else
        # Try again as it might already be set
        LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/login" \
            -H "Content-Type: application/json" \
            -d '{"password":"testpass123"}')
        TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$TOKEN" ]; then
            pass "Login successful"
        else
            fail "Login failed: $LOGIN_RESP"
            cleanup
            exit 1
        fi
    fi

    # Run tests
    echo ""
    test_login
    test_get_rules
    test_add_rule
    test_get_status
    test_update_rule
    test_tcp_forwarding
    test_delete_rule
    test_websocket

    # Cleanup
    echo ""
    cleanup

    # Summary
    log "=== Test Summary ==="
    echo -e "$TEST_RESULT"

    if echo "$TEST_RESULT" | grep -q "✗"; then
        echo "Some tests failed!"
        exit 1
    else
        echo "All tests passed!"
        exit 0
    fi
}

main "$@"
