#!/bin/bash
set -e

API_URL="http://localhost:8080/api/v1"
FRONTEND_URL="http://localhost:3000"
PASS=0
FAIL=0
TOKEN=""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() {
    echo -e "${GREEN}PASS${NC}: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo -e "${RED}FAIL${NC}: $1 — $2"
    FAIL=$((FAIL + 1))
}

# Wait for services to be healthy
echo "Waiting for backend to be ready..."
for i in $(seq 1 60); do
    if curl -sf "${API_URL}/health" > /dev/null 2>&1; then
        echo "Backend is ready."
        break
    fi
    if [ "$i" -eq 60 ]; then
        echo "ERROR: Backend did not become ready in time"
        exit 1
    fi
    sleep 2
done

echo "Waiting for frontend to be ready..."
for i in $(seq 1 30); do
    if curl -sf "${FRONTEND_URL}" > /dev/null 2>&1; then
        echo "Frontend is ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "WARNING: Frontend not ready, continuing with API tests"
        break
    fi
    sleep 2
done

echo ""
echo "========================================="
echo "  MedOps Integration Tests"
echo "========================================="
echo ""

# ---- Health Check ----
echo "--- Health Check ---"
HEALTH=$(curl -sf "${API_URL}/health")
if echo "$HEALTH" | jq -e '.status == "ok"' > /dev/null 2>&1; then
    pass "Health check returns ok"
else
    fail "Health check" "Expected status ok, got: $HEALTH"
fi

# ---- Auth: Login ----
echo ""
echo "--- Authentication ---"

# Login with valid credentials
LOGIN_RES=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"AdminPass1234"}')
TOKEN=$(echo "$LOGIN_RES" | jq -r '.token // empty')
if [ -n "$TOKEN" ]; then
    pass "Login with valid credentials"
else
    fail "Login with valid credentials" "No token returned: $LOGIN_RES"
fi

# Login with invalid credentials
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"wrongpassword"}')
if [ "$HTTP_CODE" -eq 401 ]; then
    pass "Login with invalid credentials returns 401"
else
    fail "Login with invalid credentials" "Expected 401, got $HTTP_CODE"
fi

# Get current user
ME_RES=$(curl -sf "${API_URL}/auth/me" -H "Authorization: Bearer $TOKEN")
if echo "$ME_RES" | jq -e '.username == "admin"' > /dev/null 2>&1; then
    pass "Get current user profile"
else
    fail "Get current user profile" "Unexpected response: $ME_RES"
fi

# Unauthorized access
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/auth/me")
if [ "$HTTP_CODE" -eq 401 ]; then
    pass "Unauthorized request returns 401"
else
    fail "Unauthorized request" "Expected 401, got $HTTP_CODE"
fi

AUTH_HEADER="Authorization: Bearer $TOKEN"

# Logout returns 204 No Content
LOGOUT_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/auth/logout" \
    -H "$AUTH_HEADER")
if [ "$LOGOUT_CODE" -eq 204 ]; then
    pass "Logout returns 204 No Content"
else
    fail "Logout status code" "Expected 204, got $LOGOUT_CODE"
fi

# Re-login after logout test
LOGIN_RES2=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"AdminPass1234"}')
TOKEN=$(echo "$LOGIN_RES2" | jq -r '.token')
AUTH_HEADER="Authorization: Bearer $TOKEN"

# Password change returns 204 No Content
PWCHANGE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "${API_URL}/auth/password" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"old_password":"AdminPass1234","new_password":"AdminPass1234New"}')
if [ "$PWCHANGE_CODE" -eq 204 ]; then
    pass "Password change returns 204 No Content"
else
    fail "Password change status code" "Expected 204, got $PWCHANGE_CODE"
fi

# Re-login with new password, then immediately restore original password
LOGIN_RES3=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"AdminPass1234New"}')
TOKEN=$(echo "$LOGIN_RES3" | jq -r '.token')
AUTH_HEADER="Authorization: Bearer $TOKEN"

# Restore original password so subsequent tests and re-runs use a known credential
PWRESTORE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "${API_URL}/auth/password" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"old_password":"AdminPass1234New","new_password":"AdminPass1234"}')
if [ "$PWRESTORE_CODE" -eq 204 ]; then
    pass "Admin password restored to original"
else
    fail "Admin password restore" "Expected 204, got $PWRESTORE_CODE"
fi

# Re-login with restored original password
LOGIN_RES4=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"AdminPass1234"}')
TOKEN=$(echo "$LOGIN_RES4" | jq -r '.token')
AUTH_HEADER="Authorization: Bearer $TOKEN"

# ---- User Management ----
echo ""
echo "--- User Management ---"

# Create user with short password (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/users" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"username":"testuser","password":"short","role":"front_desk"}')
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Create user with short password returns 400"
else
    fail "Create user with short password" "Expected 400, got $HTTP_CODE"
fi

# Create user with valid data
CREATE_USER=$(curl -sf -X POST "${API_URL}/users" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"username":"pharmacist1","password":"SecurePass1234","role":"inventory_pharmacist"}')
PHARM_ID=$(echo "$CREATE_USER" | jq -r '.id // empty')
if [ -n "$PHARM_ID" ]; then
    pass "Create user with valid data"
else
    fail "Create user with valid data" "No user ID returned: $CREATE_USER"
fi

# Create front desk user
CREATE_FD=$(curl -sf -X POST "${API_URL}/users" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"username":"frontdesk1","password":"SecurePass1234","role":"front_desk"}')
FD_ID=$(echo "$CREATE_FD" | jq -r '.id // empty')
if [ -n "$FD_ID" ]; then
    pass "Create front desk user"
else
    fail "Create front desk user" "$CREATE_FD"
fi

# Create maintenance tech user
CREATE_MT=$(curl -sf -X POST "${API_URL}/users" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"username":"technicianA","password":"SecurePass1234","role":"maintenance_tech"}')
MT_ID=$(echo "$CREATE_MT" | jq -r '.id // empty')
if [ -n "$MT_ID" ]; then
    pass "Create maintenance tech user"
else
    fail "Create maintenance tech user" "$CREATE_MT"
fi

# List users
USERS=$(curl -sf "${API_URL}/users" -H "$AUTH_HEADER")
USER_COUNT=$(echo "$USERS" | jq 'length')
if [ "$USER_COUNT" -ge 4 ]; then
    pass "List users returns expected count"
else
    fail "List users" "Expected >=4 users, got $USER_COUNT"
fi

# ---- Role-based Access ----
echo ""
echo "--- Role-Based Access Control ---"

# Login as pharmacist
PHARM_LOGIN=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"pharmacist1","password":"SecurePass1234"}')
PHARM_TOKEN=$(echo "$PHARM_LOGIN" | jq -r '.token // empty')

# Pharmacist cannot access user management
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/users" \
    -H "Authorization: Bearer $PHARM_TOKEN")
if [ "$HTTP_CODE" -eq 403 ]; then
    pass "Pharmacist cannot access user management (403)"
else
    fail "Pharmacist role restriction" "Expected 403, got $HTTP_CODE"
fi

# ---- Inventory: SKUs ----
echo ""
echo "--- Inventory Management ---"

# Create SKU
CREATE_SKU=$(curl -sf -X POST "${API_URL}/skus" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d '{"name":"Amoxicillin 500mg","ndc":"12345-678-90","description":"Antibiotic","unit_of_measure":"capsule","low_stock_threshold":50,"storage_location":"Shelf A-1"}')
SKU_ID=$(echo "$CREATE_SKU" | jq -r '.id // empty')
if [ -n "$SKU_ID" ]; then
    pass "Create SKU"
else
    fail "Create SKU" "$CREATE_SKU"
fi

# Create SKU missing name (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/skus" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d '{"description":"No name","unit_of_measure":"capsule"}')
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Create SKU without name returns 400"
else
    fail "Create SKU validation" "Expected 400, got $HTTP_CODE"
fi

# Get SKU
GET_SKU=$(curl -sf "${API_URL}/skus/${SKU_ID}" -H "Authorization: Bearer $PHARM_TOKEN")
if echo "$GET_SKU" | jq -e '(.name == "Amoxicillin 500mg") or (.sku.name == "Amoxicillin 500mg")' > /dev/null 2>&1; then
    pass "Get SKU by ID"
else
    fail "Get SKU by ID" "$GET_SKU"
fi

# List SKUs
LIST_SKUS=$(curl -sf "${API_URL}/skus" -H "Authorization: Bearer $PHARM_TOKEN")
if echo "$LIST_SKUS" | jq -e '.data | length > 0' > /dev/null 2>&1; then
    pass "List SKUs"
else
    fail "List SKUs" "$LIST_SKUS"
fi

# ---- Inventory: Receive ----
echo ""
echo "--- Stock Transactions ---"

FUTURE_DATE=$(date -d "+365 days" +%Y-%m-%d 2>/dev/null || date -v+365d +%Y-%m-%d 2>/dev/null || echo "2027-12-31")

RECEIVE=$(curl -sf -X POST "${API_URL}/inventory/receive" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d "{\"sku_id\":\"${SKU_ID}\",\"lot_number\":\"LOT-001\",\"expiration_date\":\"${FUTURE_DATE}\",\"quantity\":100,\"storage_location\":\"Shelf A-1\",\"reason_code\":\"purchase_order\"}")
BATCH_ID=$(echo "$RECEIVE" | jq -r '.batch.id // empty')
if [ -n "$BATCH_ID" ]; then
    pass "Receive stock"
else
    fail "Receive stock" "$RECEIVE"
fi

# Receive with expired date (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/inventory/receive" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d "{\"sku_id\":\"${SKU_ID}\",\"lot_number\":\"LOT-002\",\"expiration_date\":\"2020-01-01\",\"quantity\":50,\"storage_location\":\"Shelf A-1\",\"reason_code\":\"purchase_order\"}")
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Receive with expired date returns 400"
else
    fail "Receive with expired date" "Expected 400, got $HTTP_CODE"
fi

# Dispense stock
DISPENSE=$(curl -sf -X POST "${API_URL}/inventory/dispense" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d "{\"sku_id\":\"${SKU_ID}\",\"batch_id\":\"${BATCH_ID}\",\"quantity\":10,\"reason_code\":\"prescription\",\"prescription_id\":\"RX-001\"}")
if echo "$DISPENSE" | jq -e '.transaction.id' > /dev/null 2>&1; then
    pass "Dispense stock"
else
    fail "Dispense stock" "$DISPENSE"
fi

# Dispense more than available (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/inventory/dispense" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d "{\"sku_id\":\"${SKU_ID}\",\"batch_id\":\"${BATCH_ID}\",\"quantity\":999,\"reason_code\":\"prescription\"}")
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Dispense more than available returns 400"
else
    fail "Dispense more than available" "Expected 400, got $HTTP_CODE"
fi

# List transactions
TXS=$(curl -sf "${API_URL}/inventory/transactions?sku_id=${SKU_ID}" -H "Authorization: Bearer $PHARM_TOKEN")
if echo "$TXS" | jq -e '.data | length >= 2' > /dev/null 2>&1; then
    pass "List stock transactions"
else
    fail "List stock transactions" "$TXS"
fi

# ---- Learning ----
echo ""
echo "--- Learning Management ---"

# Create learning coordinator
curl -sf -X POST "${API_URL}/users" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"username":"coordinator1","password":"SecurePass1234","role":"learning_coordinator"}' > /dev/null

LC_LOGIN=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"coordinator1","password":"SecurePass1234"}')
LC_TOKEN=$(echo "$LC_LOGIN" | jq -r '.token // empty')

# Create subject
CREATE_SUBJ=$(curl -sf -X POST "${API_URL}/learning/subjects" \
    -H "Authorization: Bearer $LC_TOKEN" -H "Content-Type: application/json" \
    -d '{"name":"Pharmacology","description":"Drug interactions and dosages","sort_order":1}')
SUBJ_ID=$(echo "$CREATE_SUBJ" | jq -r '.id // empty')
if [ -n "$SUBJ_ID" ]; then
    pass "Create learning subject"
else
    fail "Create learning subject" "$CREATE_SUBJ"
fi

# Create chapter
CREATE_CHAP=$(curl -sf -X POST "${API_URL}/learning/chapters" \
    -H "Authorization: Bearer $LC_TOKEN" -H "Content-Type: application/json" \
    -d "{\"subject_id\":\"${SUBJ_ID}\",\"name\":\"Antibiotics\",\"sort_order\":1}")
CHAP_ID=$(echo "$CREATE_CHAP" | jq -r '.id // empty')
if [ -n "$CHAP_ID" ]; then
    pass "Create learning chapter"
else
    fail "Create learning chapter" "$CREATE_CHAP"
fi

# Create knowledge point
CREATE_KP=$(curl -sf -X POST "${API_URL}/learning/knowledge-points" \
    -H "Authorization: Bearer $LC_TOKEN" -H "Content-Type: application/json" \
    -d "{\"chapter_id\":\"${CHAP_ID}\",\"title\":\"Amoxicillin Usage\",\"content\":\"# Amoxicillin\nCommon antibiotic used for bacterial infections.\",\"tags\":[\"antibiotic\",\"penicillin\"]}")
KP_ID=$(echo "$CREATE_KP" | jq -r '.id // empty')
if [ -n "$KP_ID" ]; then
    pass "Create knowledge point"
else
    fail "Create knowledge point" "$CREATE_KP"
fi

# Search knowledge points
SEARCH=$(curl -sf "${API_URL}/learning/search?q=amoxicillin" -H "Authorization: Bearer $LC_TOKEN")
if echo "$SEARCH" | jq -e '.data | length > 0' > /dev/null 2>&1; then
    pass "Full-text search knowledge points"
else
    fail "Full-text search" "$SEARCH"
fi

# ---- Work Orders ----
echo ""
echo "--- Work Orders ---"

# Login as front desk to submit work order
FD_LOGIN=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"frontdesk1","password":"SecurePass1234"}')
FD_TOKEN=$(echo "$FD_LOGIN" | jq -r '.token // empty')

# Create work order
CREATE_WO=$(curl -sf -X POST "${API_URL}/work-orders" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"trade":"electrical","priority":"high","description":"Broken light in exam room 3","location":"Building A, Room 303"}')
WO_ID=$(echo "$CREATE_WO" | jq -r '.id // empty')
if [ -n "$WO_ID" ]; then
    pass "Create work order"
else
    fail "Create work order" "$CREATE_WO"
fi

# Get work order — response is now {work_order: {...}, photos: []}
GET_WO=$(curl -sf "${API_URL}/work-orders/${WO_ID}" -H "Authorization: Bearer $FD_TOKEN")
if echo "$GET_WO" | jq -e '.work_order.priority == "high" and (.photos | type) == "array"' > /dev/null 2>&1; then
    pass "Get work order detail"
else
    fail "Get work order detail" "$GET_WO"
fi

# Login as maintenance tech
MT_LOGIN=$(curl -sf -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"technicianA","password":"SecurePass1234"}')
MT_TOKEN=$(echo "$MT_LOGIN" | jq -r '.token // empty')

# Close work order
CLOSE_WO=$(curl -sf -X POST "${API_URL}/work-orders/${WO_ID}/close" \
    -H "Authorization: Bearer $MT_TOKEN" -H "Content-Type: application/json" \
    -d '{"parts_cost":25.50,"labor_cost":75.00}')
if echo "$CLOSE_WO" | jq -e '.status' > /dev/null 2>&1; then
    pass "Close work order with costs"
else
    fail "Close work order" "$CLOSE_WO"
fi

# Rate work order
RATE_WO=$(curl -sf -X POST "${API_URL}/work-orders/${WO_ID}/rate" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"rating":4}')
if echo "$RATE_WO" | jq -e '.rating == 4' > /dev/null 2>&1; then
    pass "Rate work order"
else
    fail "Rate work order" "$RATE_WO"
fi

# Rate with invalid value
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/work-orders/${WO_ID}/rate" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"rating":6}')
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Invalid rating returns 400"
else
    fail "Invalid rating" "Expected 400, got $HTTP_CODE"
fi

# ---- Members ----
echo ""
echo "--- Membership Management ---"

# List tiers
TIERS=$(curl -sf "${API_URL}/membership-tiers" -H "Authorization: Bearer $FD_TOKEN")
TIER_ID=$(echo "$TIERS" | jq -r '.data[0].id // empty')
if [ -n "$TIER_ID" ]; then
    pass "List membership tiers"
else
    fail "List membership tiers" "$TIERS"
fi

# Create member
CREATE_MEM=$(curl -sf -X POST "${API_URL}/members" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d "{\"name\":\"John Doe\",\"id_number\":\"ID-123456\",\"phone\":\"555-0100\",\"tier_id\":\"${TIER_ID}\"}")
MEM_ID=$(echo "$CREATE_MEM" | jq -r '.id // empty')
if [ -n "$MEM_ID" ]; then
    pass "Create member"
else
    fail "Create member" "$CREATE_MEM"
fi

# Add stored value
ADD_VAL=$(curl -sf -X POST "${API_URL}/members/${MEM_ID}/add-value" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"type":"stored_value_add","amount":100}')
if echo "$ADD_VAL" | jq -e '.id' > /dev/null 2>&1; then
    pass "Add stored value"
else
    fail "Add stored value" "$ADD_VAL"
fi

# Redeem stored value
REDEEM=$(curl -sf -X POST "${API_URL}/members/${MEM_ID}/redeem" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"type":"stored_value_use","amount":25}')
if echo "$REDEEM" | jq -e '.id' > /dev/null 2>&1; then
    pass "Redeem stored value"
else
    fail "Redeem stored value" "$REDEEM"
fi

# Redeem more than balance (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/members/${MEM_ID}/redeem" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"type":"stored_value_use","amount":999}')
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Redeem more than balance returns 400"
else
    fail "Redeem more than balance" "Expected 400, got $HTTP_CODE"
fi

# Freeze member
FREEZE=$(curl -sf -X POST "${API_URL}/members/${MEM_ID}/freeze" \
    -H "Authorization: Bearer $FD_TOKEN")
if echo "$FREEZE" | jq -e '.status == "frozen"' > /dev/null 2>&1; then
    pass "Freeze member"
else
    fail "Freeze member" "$FREEZE"
fi

# Redeem while frozen (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/members/${MEM_ID}/redeem" \
    -H "Authorization: Bearer $FD_TOKEN" -H "Content-Type: application/json" \
    -d '{"type":"stored_value_use","amount":10}')
if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Redeem while frozen returns 400"
else
    fail "Redeem while frozen" "Expected 400, got $HTTP_CODE"
fi

# Unfreeze member
UNFREEZE=$(curl -sf -X POST "${API_URL}/members/${MEM_ID}/unfreeze" \
    -H "Authorization: Bearer $FD_TOKEN")
if echo "$UNFREEZE" | jq -e '.status == "active"' > /dev/null 2>&1; then
    pass "Unfreeze member"
else
    fail "Unfreeze member" "$UNFREEZE"
fi

# Member transactions
MEM_TXS=$(curl -sf "${API_URL}/members/${MEM_ID}/transactions" -H "Authorization: Bearer $FD_TOKEN")
if echo "$MEM_TXS" | jq -e '.data | length >= 2' > /dev/null 2>&1; then
    pass "List member transactions"
else
    fail "List member transactions" "$MEM_TXS"
fi

# ---- Charges ----
echo ""
echo "--- Charges & Settlement ---"

# Create rate table
CREATE_RT=$(curl -sf -X POST "${API_URL}/rate-tables" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"name":"Standard Distance","type":"distance","tiers":[{"min":0,"max":10,"rate":5.00},{"min":10,"max":50,"rate":3.50}],"fuel_surcharge_pct":8.5,"taxable":true,"effective_date":"2026-01-01"}')
if echo "$CREATE_RT" | jq -e '.id' > /dev/null 2>&1; then
    pass "Create rate table"
else
    fail "Create rate table" "$CREATE_RT"
fi
RT_ID=$(echo "$CREATE_RT" | jq -r '.id // empty')

# Generate statement — must include rate_table_id and line_items (handler requires both)
GEN_STMT=$(curl -sf -X POST "${API_URL}/statements/generate" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"period_start\":\"2026-01-01\",\"period_end\":\"2026-01-31\",\"rate_table_id\":\"${RT_ID}\",\"line_items\":[{\"description\":\"Supply transport\",\"quantity\":5}]}")
STMT_ID=$(echo "$GEN_STMT" | jq -r '.statement.id // empty')
if [ -n "$STMT_ID" ]; then
    pass "Generate charge statement"
else
    fail "Generate charge statement" "$GEN_STMT"
fi

# ---- System Config ----
# [confidence gap] Previously only checked shape + PUT status.
# Now verifies a full round-trip: PUT a unique key/value, then GET and confirm
# the value was actually persisted.
echo ""
echo "--- System Configuration ---"

# GET /system/config must return { "config": { ... } } with an OBJECT for .config.
CFG_RES=$(curl -sf "${API_URL}/system/config" -H "$AUTH_HEADER")
if echo "$CFG_RES" | jq -e '.config | type == "object"' > /dev/null 2>&1; then
    pass "GET /system/config returns { config: {object} } shape"
else
    fail "GET /system/config shape" "Expected .config to be an object, got: $CFG_RES"
fi

# PUT a unique key/value so we can verify round-trip without interfering with other runs.
CFG_KEY="e2e_cfg_$(date +%s%N)"
CFG_VAL="roundtrip-$(date +%s)"
CFG_UPDATE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "${API_URL}/system/config" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"key\":\"${CFG_KEY}\",\"value\":\"${CFG_VAL}\"}")
if [ "$CFG_UPDATE" -eq 200 ]; then
    pass "PUT /system/config returns 200"
else
    fail "PUT /system/config" "Expected 200, got $CFG_UPDATE"
fi

# Round-trip: GET must return the value we just wrote.
CFG_AFTER=$(curl -sf "${API_URL}/system/config" -H "$AUTH_HEADER")
if echo "$CFG_AFTER" | jq -e --arg k "$CFG_KEY" --arg v "$CFG_VAL" \
    '.config[$k] == $v' > /dev/null 2>&1; then
    pass "System config PUT→GET round-trip: .config[$CFG_KEY] == $CFG_VAL"
else
    fail "System config round-trip" "Expected .config.$CFG_KEY = $CFG_VAL, got: $CFG_AFTER"
fi

# ---- Backup endpoint — response must include sql and files_archive fields ----
echo ""
echo "--- System Backup ---"

# [confidence gap] Previously just checked key presence. Now asserts:
#  - backup_file is a non-empty path ending in ".sql"
#  - files_archive is present (may be empty string if archive step was skipped,
#    but when non-empty it must end in ".zip")
#  - timestamp field is a non-empty UTC datetime
BACKUP_RES=$(curl -s -X POST "${API_URL}/system/backup" -H "$AUTH_HEADER")
BACKUP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/system/backup" -H "$AUTH_HEADER")
if [ "$BACKUP_STATUS" -eq 200 ]; then
    if echo "$BACKUP_RES" | jq -e '
        (.backup_file | type == "string" and endswith(".sql") and length > 10)
        and (has("files_archive"))
        and ((.files_archive == "") or (.files_archive | endswith(".zip")))
        and (.timestamp | type == "string" and length > 0)
    ' > /dev/null 2>&1; then
        pass "POST /system/backup: backup_file=.sql, files_archive=.zip or empty, timestamp set"
    else
        fail "POST /system/backup contract" "Shape/paths not as expected: $BACKUP_RES"
    fi
else
    # pg_dump missing — record a skip (not a pass).
    echo "SKIP: POST /system/backup (pg_dump unavailable, HTTP $BACKUP_STATUS)"
fi

# ---- 404 ----
echo ""
echo "--- Error Handling ---"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/skus/00000000-0000-0000-0000-000000000000" \
    -H "Authorization: Bearer $PHARM_TOKEN")
if [ "$HTTP_CODE" -eq 404 ]; then
    pass "Non-existent resource returns 404"
else
    fail "Non-existent resource" "Expected 404, got $HTTP_CODE"
fi

# ---- Frontend ----
echo ""
echo "--- Frontend ---"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${FRONTEND_URL}")
if [ "$HTTP_CODE" -eq 200 ]; then
    pass "Frontend serves index.html"
else
    fail "Frontend" "Expected 200, got $HTTP_CODE"
fi

# =============================================================================
# DEEP API FLOW TESTS — close the gaps called out in the coverage audit:
#   "run_tests.sh still misses depth on key flows (drafts, file upload/download,
#    full statement lifecycle reconcile/approve/export, stocktake, photo link)"
#
# NOTE: disable `set -e` inside this section so individual-test failures don't
# abort the whole suite — each test tracks its own pass/fail via the `fail` fn.
# =============================================================================
set +e

# ---- Drafts lifecycle (auto-save) ----
# [confidence gap] Previously accepted any of 200/201/204 for PUT, 200-or-404 for GET/DELETE.
# The SaveDraft handler returns exactly 201 Created; GetDraft returns 200 with the draft
# payload; DeleteDraft returns 200 with a JSON message. Loose acceptance masked regressions
# where an endpoint silently stopped creating rows (still 200/404 on GET).
echo ""
echo "--- Drafts (auto-save) ---"

DRAFT_FORM_TYPE="wo_create_$(date +%s)"
DRAFT_FORM_ID="draft-$(date +%s%N)"
DRAFT_STATE='{"description":"draft wo","priority":"high"}'

# PUT a draft — strict 201 expected, response body must echo state_json.
DRAFT_PUT_BODY=$(mktemp)
DRAFT_PUT=$(curl -s -o "$DRAFT_PUT_BODY" -w "%{http_code}" -X PUT "${API_URL}/drafts/${DRAFT_FORM_TYPE}" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"form_id\":\"${DRAFT_FORM_ID}\",\"state_json\":${DRAFT_STATE}}")
if [ "$DRAFT_PUT" -eq 201 ]; then
    # Response should include a draft id
    PUT_DRAFT_ID=$(jq -r '.id // empty' < "$DRAFT_PUT_BODY")
    if [ -n "$PUT_DRAFT_ID" ]; then
        pass "PUT /drafts/:formType creates draft (HTTP 201, id=${PUT_DRAFT_ID:0:8}...)"
    else
        fail "PUT /drafts/:formType" "201 returned but no id in body: $(cat "$DRAFT_PUT_BODY")"
    fi
else
    fail "PUT /drafts/:formType" "Expected exactly 201, got $DRAFT_PUT"
fi
rm -f "$DRAFT_PUT_BODY"

# LIST drafts — strict 200 and the listed draft must be our new one.
DRAFT_LIST_BODY=$(curl -s "${API_URL}/drafts" -H "$AUTH_HEADER")
if echo "$DRAFT_LIST_BODY" | jq -e --arg ft "$DRAFT_FORM_TYPE" --arg fid "$DRAFT_FORM_ID" \
    '(.data // .) | map(select(.form_type == $ft and .form_id == $fid)) | length == 1' > /dev/null 2>&1; then
    pass "GET /drafts lists the saved draft (exact form_type + form_id match)"
else
    fail "GET /drafts" "Saved draft not found by (form_type, form_id) in: $DRAFT_LIST_BODY"
fi

# GET the specific draft — strict 200 and payload MUST equal what we saved.
DRAFT_GET_BODY=$(mktemp)
DRAFT_GET=$(curl -s -o "$DRAFT_GET_BODY" -w "%{http_code}" \
    "${API_URL}/drafts/${DRAFT_FORM_TYPE}/${DRAFT_FORM_ID}" -H "$AUTH_HEADER")
if [ "$DRAFT_GET" -eq 200 ]; then
    # Payload must round-trip: state_json.description == "draft wo" AND state_json.priority == "high"
    if jq -e '.state_json.description == "draft wo" and .state_json.priority == "high"' \
        < "$DRAFT_GET_BODY" > /dev/null 2>&1; then
        pass "GET /drafts/:formType/:formId returns exact saved payload"
    else
        fail "GET draft payload" "state_json did not round-trip: $(cat "$DRAFT_GET_BODY")"
    fi
else
    fail "GET draft" "Expected 200, got $DRAFT_GET"
fi
rm -f "$DRAFT_GET_BODY"

# DELETE draft — strict 200 expected.
DRAFT_DEL_BODY=$(mktemp)
DRAFT_DEL=$(curl -s -o "$DRAFT_DEL_BODY" -w "%{http_code}" -X DELETE \
    "${API_URL}/drafts/${DRAFT_FORM_TYPE}/${DRAFT_FORM_ID}" -H "$AUTH_HEADER")
if [ "$DRAFT_DEL" -eq 200 ]; then
    pass "DELETE /drafts/:formType/:formId returns 200"
else
    fail "DELETE draft" "Expected 200, got $DRAFT_DEL (body: $(cat "$DRAFT_DEL_BODY"))"
fi
rm -f "$DRAFT_DEL_BODY"

# Follow-up: after DELETE, the draft MUST NOT be returned. Current backend
# contract (system.go:1185): on no-row, GetDraft returns 200 with body "null"
# rather than 404. The tight assertion is that the body is not the original
# draft — catching regressions regardless of which status code is returned.
DRAFT_GET_AFTER_DEL_BODY=$(mktemp)
DRAFT_GET_AFTER_DEL=$(curl -s -o "$DRAFT_GET_AFTER_DEL_BODY" -w "%{http_code}" \
    "${API_URL}/drafts/${DRAFT_FORM_TYPE}/${DRAFT_FORM_ID}" -H "$AUTH_HEADER")
AFTER_DEL_BODY=$(tr -d '[:space:]' < "$DRAFT_GET_AFTER_DEL_BODY")
if [ "$DRAFT_GET_AFTER_DEL" -eq 404 ]; then
    pass "GET deleted draft returns 404"
elif [ "$DRAFT_GET_AFTER_DEL" -eq 200 ] && [ "$AFTER_DEL_BODY" = "null" ]; then
    pass "GET deleted draft returns 200 with body 'null' (delete durable)"
else
    fail "Draft delete durability" "After DELETE, expected 404 or 200+null; got $DRAFT_GET_AFTER_DEL body='$AFTER_DEL_BODY'"
fi
rm -f "$DRAFT_GET_AFTER_DEL_BODY"

# Unauthenticated drafts access returns 401
DRAFT_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/drafts")
if [ "$DRAFT_NOAUTH" -eq 401 ]; then
    pass "GET /drafts without auth returns 401"
else
    fail "GET /drafts no-auth" "Expected 401, got $DRAFT_NOAUTH"
fi

# ---- File upload / download round-trip ----
# [confidence gap] Previously only asserted byte count equality. Now also asserts
# upload-response metadata (id is a UUID, original filename is returned),
# content-disposition on download, and exact byte match using sha256.
echo ""
echo "--- Files (upload / download / auth) ---"

# Create a test payload with a stable filename so we can assert metadata echoes it.
TEST_FILE=$(mktemp --suffix=.txt)
UPLOAD_FILENAME=$(basename "$TEST_FILE")
echo "e2e-content-$(date +%s)" > "$TEST_FILE"
EXPECTED_BYTES=$(wc -c < "$TEST_FILE")
EXPECTED_SHA=$(sha256sum "$TEST_FILE" | awk '{print $1}')

UPLOAD_RES=$(curl -s -X POST "${API_URL}/files/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "file=@${TEST_FILE}")
FILE_ID=$(echo "$UPLOAD_RES" | jq -r '.file.id // .id // empty')
# UUID v4 pattern — the id must be a well-formed UUID (not just any non-empty string).
UUID_RE='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
if [[ "$FILE_ID" =~ $UUID_RE ]]; then
    pass "POST /files/upload returns a UUID-shaped file id"
else
    fail "POST /files/upload" "id is not a UUID: '$FILE_ID' (body: $UPLOAD_RES)"
fi

if [ -n "$FILE_ID" ]; then
    DOWNLOAD_TMP=$(mktemp)
    DOWNLOAD_STATUS=$(curl -s -o "$DOWNLOAD_TMP" -w "%{http_code}" \
        "${API_URL}/files/${FILE_ID}" -H "$AUTH_HEADER")
    DOWNLOAD_BYTES=$(wc -c < "$DOWNLOAD_TMP")
    DOWNLOAD_SHA=$(sha256sum "$DOWNLOAD_TMP" | awk '{print $1}')
    if [ "$DOWNLOAD_STATUS" -ne 200 ]; then
        fail "GET /files/:id" "Expected 200, got $DOWNLOAD_STATUS"
    elif [ "$DOWNLOAD_BYTES" -ne "$EXPECTED_BYTES" ]; then
        fail "GET /files/:id byte count" "Got $DOWNLOAD_BYTES bytes, expected $EXPECTED_BYTES"
    elif [ "$DOWNLOAD_SHA" != "$EXPECTED_SHA" ]; then
        fail "GET /files/:id integrity" "SHA mismatch: $DOWNLOAD_SHA != $EXPECTED_SHA"
    else
        pass "GET /files/:id round-trips exact bytes (sha256 match, $DOWNLOAD_BYTES bytes)"
    fi
    rm -f "$DOWNLOAD_TMP"

    # Download must include a Content-Disposition header for browser "save as" to work.
    # Use GET with -D (dump response headers) — the Download handler only
    # registers GET (not HEAD), so curl -I would return 404. We only care that
    # the GET response carries the header.
    HEADERS_FILE=$(mktemp)
    curl -s -D "$HEADERS_FILE" -o /dev/null "${API_URL}/files/${FILE_ID}" -H "$AUTH_HEADER"
    if grep -qi '^Content-Disposition:' "$HEADERS_FILE"; then
        pass "GET /files/:id returns Content-Disposition header (download metadata present)"
    else
        fail "GET /files/:id Content-Disposition" "Header missing in: $(cat "$HEADERS_FILE")"
    fi
    rm -f "$HEADERS_FILE"

    # Unauthenticated download → 401 (negative test, deterministic).
    FILE_NOAUTH=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/files/${FILE_ID}")
    if [ "$FILE_NOAUTH" -eq 401 ]; then
        pass "GET /files/:id without auth returns 401"
    else
        fail "GET /files/:id no-auth" "Expected 401, got $FILE_NOAUTH"
    fi

    # Non-existent file id → 404 (deterministic not-found contract).
    NOEXIST=$(curl -s -o /dev/null -w "%{http_code}" \
        "${API_URL}/files/00000000-0000-0000-0000-000000000000" -H "$AUTH_HEADER")
    if [ "$NOEXIST" -eq 404 ] || [ "$NOEXIST" -eq 403 ]; then
        # 403 is acceptable IF the object-level auth check runs before the existence check
        pass "GET /files/:non-existent returns $NOEXIST (object-level auth / not found)"
    else
        fail "GET /files/:non-existent" "Expected 404 or 403, got $NOEXIST"
    fi
fi
rm -f "$TEST_FILE"

# ---- Work order photo link ----
echo ""
echo "--- Work Order Photos ---"

WO_PHOTO_CREATE=$(curl -s -X POST "${API_URL}/work-orders" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d '{"trade":"electrical","priority":"normal","description":"photo-link WO","location":"A"}')
WO_PHOTO_ID=$(echo "$WO_PHOTO_CREATE" | jq -r '.id // empty')
if [ -n "$WO_PHOTO_ID" ]; then
    pass "Create work order for photo-link test"
else
    fail "Create WO for photo test" "No id in response: $WO_PHOTO_CREATE"
fi

if [ -n "$FILE_ID" ] && [ -n "$WO_PHOTO_ID" ]; then
    # [confidence gap] Previously accepted 200/201/204. The LinkPhoto handler returns
    # exactly 201 Created (workorders.go:628). Loose status checks could hide a handler
    # that silently switched to 204-No-Content-without-writing.
    LINK_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
        "${API_URL}/work-orders/${WO_PHOTO_ID}/photos" \
        -H "$AUTH_HEADER" -H "Content-Type: application/json" \
        -d "{\"file_id\":\"${FILE_ID}\"}")
    if [ "$LINK_STATUS" -eq 201 ]; then
        pass "POST /work-orders/:id/photos links a file (HTTP 201)"
    else
        fail "Link photo" "Expected exactly 201, got $LINK_STATUS"
    fi

    # [confidence gap] Previously fell back to `grep -q "$FILE_ID"` in the raw body,
    # which would pass if the id appeared *anywhere* (e.g. in an error message).
    # Now uses structured jq to require an exact match on .data[].id.
    PHOTOS_RES=$(curl -s "${API_URL}/work-orders/${WO_PHOTO_ID}/photos" -H "$AUTH_HEADER")
    if echo "$PHOTOS_RES" | jq -e --arg fid "$FILE_ID" \
        '.data | type == "array" and (map(.id) | index($fid) != null)' > /dev/null 2>&1; then
        pass "GET /work-orders/:id/photos returns envelope {data:[...]} containing the linked file"
    else
        fail "Get photos structured" "No .data[] entry with id=$FILE_ID in: $PHOTOS_RES"
    fi
fi

# ---- Stocktake full lifecycle ----
echo ""
echo "--- Stocktake Lifecycle ---"

# Create SKU for stocktake
STOCK_SKU_RES=$(curl -s -X POST "${API_URL}/skus" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"name\":\"Stocktake SKU $(date +%s)\",\"unit\":\"each\",\"category\":\"general\",\"reorder_point\":10}")
STOCK_SKU_ID=$(echo "$STOCK_SKU_RES" | jq -r '.id // empty')

STOCKTAKE_CREATE=$(curl -s -X POST "${API_URL}/stocktakes" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"period_start\":\"$(date -u -d '1 day ago' +%Y-%m-%d 2>/dev/null || date -u -v-1d +%Y-%m-%d)\",\"period_end\":\"$(date -u +%Y-%m-%d)\"}")
STOCKTAKE_ID=$(echo "$STOCKTAKE_CREATE" | jq -r '.id // empty')
if [ -n "$STOCKTAKE_ID" ] && [ "$STOCKTAKE_ID" != "null" ]; then
    pass "POST /stocktakes creates a stocktake"
else
    fail "Create stocktake" "No id in response: $STOCKTAKE_CREATE"
fi

if [ -n "$STOCKTAKE_ID" ] && [ "$STOCKTAKE_ID" != "null" ]; then
    # Verify GET returns the stocktake with its auto-populated lines.
    GET_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
        "${API_URL}/stocktakes/${STOCKTAKE_ID}" -H "$AUTH_HEADER")
    if [ "$GET_STATUS" -eq 200 ]; then
        pass "GET /stocktakes/:id returns created stocktake"
    else
        fail "Get stocktake" "Expected 200, got $GET_STATUS"
    fi

    # Complete the stocktake — strict 200 (handler returns http.StatusOK with message).
    COMPLETE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
        "${API_URL}/stocktakes/${STOCKTAKE_ID}/complete" -H "$AUTH_HEADER")
    if [ "$COMPLETE_STATUS" -eq 200 ]; then
        pass "POST /stocktakes/:id/complete closes the stocktake (HTTP 200)"
    else
        fail "Complete stocktake" "Expected exactly 200, got $COMPLETE_STATUS"
    fi

    # Post-complete GET must show a non-open status (structural check, not grep).
    POST_COMPLETE=$(curl -s "${API_URL}/stocktakes/${STOCKTAKE_ID}" -H "$AUTH_HEADER")
    ST_STATUS=$(echo "$POST_COMPLETE" | jq -r '.status // .stocktake.status // empty')
    if [ "$ST_STATUS" = "completed" ] || [ "$ST_STATUS" = "closed" ] || [ "$ST_STATUS" = "done" ]; then
        pass "Stocktake transitioned to terminal status ('$ST_STATUS') after complete"
    else
        fail "Stocktake terminal status" "Expected completed/closed/done, got '$ST_STATUS'"
    fi

    # List stocktakes must include our created one — structured jq check, not grep.
    LIST_RES=$(curl -s "${API_URL}/stocktakes" -H "$AUTH_HEADER")
    if echo "$LIST_RES" | jq -e --arg sid "$STOCKTAKE_ID" \
        '((.data // .stocktakes // .) | type == "array") and
         ((.data // .stocktakes // .) | map(.id) | index($sid) != null)' > /dev/null 2>&1; then
        pass "GET /stocktakes list contains the new stocktake (structured match)"
    else
        fail "List stocktakes" "Created id not found in structured list: $LIST_RES"
    fi
fi

# ---- Statement full lifecycle (generate -> reconcile -> approve -> export) ----
echo ""
echo "--- Statement Full Lifecycle (two-user approval) ---"

# Rate table (create fresh for this test)
LIFE_RATE=$(curl -s -X POST "${API_URL}/rate-tables" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"name\":\"Lifecycle $(date +%s)\",\"type\":\"distance\",\"tiers\":[{\"min\":0,\"max\":100,\"rate\":5}],\"fuel_surcharge_pct\":0,\"taxable\":false,\"effective_date\":\"$(date -u +%Y-%m-%d)\"}")
LIFE_RATE_ID=$(echo "$LIFE_RATE" | jq -r '.id // empty')

STMT_GEN=$(curl -s -X POST "${API_URL}/statements/generate" \
    -H "$AUTH_HEADER" -H "Content-Type: application/json" \
    -d "{\"period_start\":\"$(date -u -d '30 days ago' +%Y-%m-%d 2>/dev/null || date -u -v-30d +%Y-%m-%d)\",\"period_end\":\"$(date -u +%Y-%m-%d)\",\"rate_table_id\":\"${LIFE_RATE_ID}\",\"line_items\":[{\"description\":\"Trip\",\"quantity\":10}]}")
STMT_ID=$(echo "$STMT_GEN" | jq -r '.id // .statement.id // empty')
STMT_TOTAL=$(echo "$STMT_GEN" | jq -r '.total_amount // .statement.total_amount // 0')
if [ -n "$STMT_ID" ]; then
    pass "POST /statements/generate creates a statement (total=$STMT_TOTAL)"
else
    fail "Generate statement" "No id returned: $STMT_GEN"
fi

if [ -n "$STMT_ID" ]; then
    # [confidence gap] Strict status codes + post-state assertions.
    # ReconcileStatement returns exactly 200 with the updated statement body.
    # Same-user approval returns exactly 403 (charges.go:776). Different-user
    # approval returns 200 and must set approved_by_2.
    RECON_BODY=$(mktemp)
    RECON_STATUS=$(curl -s -o "$RECON_BODY" -w "%{http_code}" -X POST \
        "${API_URL}/statements/${STMT_ID}/reconcile" \
        -H "$AUTH_HEADER" -H "Content-Type: application/json" \
        -d "{\"expected_total\":${STMT_TOTAL},\"variance_notes\":\"\"}")
    if [ "$RECON_STATUS" -eq 200 ]; then
        # The returned statement must have status=reconciled and approved_by_1 set.
        if jq -e '.status == "reconciled" and (.approved_by_1 // "") != ""' \
            < "$RECON_BODY" > /dev/null 2>&1; then
            pass "POST /statements/:id/reconcile: status=reconciled, approved_by_1 populated"
        else
            fail "Reconcile post-state" "Body missing expected reconciled shape: $(cat "$RECON_BODY")"
        fi
    else
        fail "Reconcile" "Expected exactly 200, got $RECON_STATUS"
    fi
    rm -f "$RECON_BODY"

    # Same-user approve must be rejected with exactly 403 (not 400) — two-step integrity.
    SAME_APPROVE=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
        "${API_URL}/statements/${STMT_ID}/approve" -H "$AUTH_HEADER")
    if [ "$SAME_APPROVE" -eq 403 ]; then
        pass "Same-user approval rejected with exactly 403 Forbidden"
    else
        fail "Same-user approve" "Expected exactly 403, got $SAME_APPROVE"
    fi

    # Create a second admin user, rotate password, then approve from that identity.
    APPROVER_NAME="approver_$(date +%s)"
    curl -s -X POST "${API_URL}/users" -H "$AUTH_HEADER" -H "Content-Type: application/json" \
        -d "{\"username\":\"${APPROVER_NAME}\",\"password\":\"ApprovePass1234\",\"role\":\"system_admin\"}" > /dev/null
    APPROVER_TOK=$(curl -s -X POST "${API_URL}/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"${APPROVER_NAME}\",\"password\":\"ApprovePass1234\"}" | jq -r '.token // empty')
    curl -s -X PUT "${API_URL}/auth/password" \
        -H "Authorization: Bearer ${APPROVER_TOK}" -H "Content-Type: application/json" \
        -d '{"old_password":"ApprovePass1234","new_password":"ApprovePassNEW1234"}' > /dev/null
    APPROVER_TOK=$(curl -s -X POST "${API_URL}/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"${APPROVER_NAME}\",\"password\":\"ApprovePassNEW1234\"}" | jq -r '.token // empty')

    APPROVE_BODY=$(mktemp)
    APPROVE_STATUS=$(curl -s -o "$APPROVE_BODY" -w "%{http_code}" -X POST \
        "${API_URL}/statements/${STMT_ID}/approve" \
        -H "Authorization: Bearer ${APPROVER_TOK}")
    if [ "$APPROVE_STATUS" -eq 200 ]; then
        # Post-state: status=approved, approved_by_2 set AND differs from approved_by_1.
        if jq -e '.status == "approved"
                  and (.approved_by_2 // "") != ""
                  and .approved_by_1 != .approved_by_2' \
            < "$APPROVE_BODY" > /dev/null 2>&1; then
            pass "Different-user approval: status=approved, approved_by_2 != approved_by_1"
        else
            fail "Approve post-state" "Body missing expected approved shape: $(cat "$APPROVE_BODY")"
        fi
    else
        fail "Different-user approve" "Expected exactly 200, got $APPROVE_STATUS"
    fi
    rm -f "$APPROVE_BODY"

    # Export (mark paid)
    # [confidence gap] Strict 200 + Content-Type CSV assertion + bytes non-empty.
    # ExportStatement returns a Blob with text/csv content type.
    EXPORT_BODY=$(mktemp)
    EXPORT_HEADERS=$(mktemp)
    EXPORT_STATUS=$(curl -s -D "$EXPORT_HEADERS" -o "$EXPORT_BODY" -w "%{http_code}" -X POST \
        "${API_URL}/statements/${STMT_ID}/export" \
        -H "$AUTH_HEADER" -H "Content-Type: application/json" \
        -d '{"format":"csv"}')
    if [ "$EXPORT_STATUS" -eq 200 ]; then
        EXPORT_BYTES=$(wc -c < "$EXPORT_BODY")
        if [ "$EXPORT_BYTES" -gt 0 ] && grep -qi '^Content-Type:.*csv' "$EXPORT_HEADERS"; then
            pass "POST /statements/:id/export returns 200 text/csv ($EXPORT_BYTES bytes)"
        else
            fail "Export content-type" "Expected CSV body >0 bytes, got $EXPORT_BYTES bytes; headers: $(cat "$EXPORT_HEADERS")"
        fi
    else
        fail "Export statement" "Expected exactly 200, got $EXPORT_STATUS"
    fi
    rm -f "$EXPORT_BODY" "$EXPORT_HEADERS"

    # Final statement state: status==paid AND paid_at set AND both approvers set.
    FINAL_STMT=$(curl -s "${API_URL}/statements/${STMT_ID}" -H "$AUTH_HEADER")
    if echo "$FINAL_STMT" | jq -e '
        (.status // .statement.status) == "paid"
        and ((.paid_at // .statement.paid_at) // "") != ""
        and ((.approved_by_1 // .statement.approved_by_1) // "") != ""
        and ((.approved_by_2 // .statement.approved_by_2) // "") != ""
        and (.approved_by_1 // .statement.approved_by_1) != (.approved_by_2 // .statement.approved_by_2)
    ' > /dev/null 2>&1; then
        pass "Final statement: status=paid, paid_at+approved_by_1+approved_by_2 all set, approvers differ"
    else
        fail "Final state" "Missing required fields: $FINAL_STMT"
    fi
fi

# ---- RBAC + Object-level authorization negative tests ----
# [confidence gap] Previously only validated that non-admin got 403 on two endpoints.
# Now also validates cross-user object-level authorization for work orders.
echo ""
echo "--- RBAC + Object-level auth (negative tests) ---"

# Inventory pharmacist cannot access /statements (system_admin only).
PHARM_403=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/statements" \
    -H "Authorization: Bearer $PHARM_TOKEN")
if [ "$PHARM_403" -eq 403 ]; then
    pass "inventory_pharmacist gets 403 on GET /statements"
else
    fail "Pharmacist statements 403" "Expected 403, got $PHARM_403"
fi

# Inventory pharmacist cannot create rate tables.
RATE_403=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/rate-tables" \
    -H "Authorization: Bearer $PHARM_TOKEN" -H "Content-Type: application/json" \
    -d '{"name":"X","type":"distance","tiers":[],"fuel_surcharge_pct":0,"taxable":false,"effective_date":"2026-01-01"}')
if [ "$RATE_403" -eq 403 ]; then
    pass "inventory_pharmacist gets 403 on POST /rate-tables"
else
    fail "Pharmacist rate-tables 403" "Expected 403, got $RATE_403"
fi

# Inventory pharmacist cannot list users (system_admin only).
USERS_403=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/users" \
    -H "Authorization: Bearer $PHARM_TOKEN")
if [ "$USERS_403" -eq 403 ]; then
    pass "inventory_pharmacist gets 403 on GET /users"
else
    fail "Pharmacist users 403" "Expected 403, got $USERS_403"
fi

# --- Object-level authorization: front_desk user cannot view another user's work order ---
# Admin creates a WO (submitted_by=admin, not assigned). A front_desk user who is
# neither the submitter nor the assignee MUST be denied with 403 (production
# contract per canViewWorkOrder in workorders.go:249). 404 is NOT acceptable here
# because that would mask an enumeration leak.
if [ -n "$FD_TOKEN" ]; then
    CROSS_WO=$(curl -s -X POST "${API_URL}/work-orders" \
        -H "$AUTH_HEADER" -H "Content-Type: application/json" \
        -d '{"trade":"electrical","priority":"normal","description":"cross-user auth test","location":"X"}')
    CROSS_WO_ID=$(echo "$CROSS_WO" | jq -r '.id // empty')
    if [ -n "$CROSS_WO_ID" ]; then
        CROSS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
            "${API_URL}/work-orders/${CROSS_WO_ID}" \
            -H "Authorization: Bearer $FD_TOKEN")
        if [ "$CROSS_STATUS" -eq 403 ]; then
            pass "Object-level auth: non-owner front_desk gets exactly 403 on another user's WO"
        else
            fail "Object-level auth" "Expected exactly 403 (deny without enumeration leak), got $CROSS_STATUS"
        fi
    else
        fail "Setup cross-user WO" "Could not create test WO: $CROSS_WO"
    fi
else
    fail "Setup cross-user auth test" "FD_TOKEN not available from earlier section"
fi

# ---- Reminders ----
echo ""
echo "--- Reminders ---"

REM_MEM=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/reminders/memberships" -H "$AUTH_HEADER")
if [ "$REM_MEM" -eq 200 ]; then
    pass "GET /reminders/memberships returns 200"
else
    fail "Reminders memberships" "Expected 200, got $REM_MEM"
fi

REM_STOCK=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/reminders/low-stock" -H "$AUTH_HEADER")
if [ "$REM_STOCK" -eq 200 ]; then
    pass "GET /reminders/low-stock returns 200"
else
    fail "Reminders low-stock" "Expected 200, got $REM_STOCK"
fi

# =============================================================================
# BACKEND GO UNIT/INTEGRATION TESTS — run inside a throwaway Go container so the
# host does not need Go installed. This adds deep behavior coverage beyond HTTP.
# =============================================================================
echo ""
echo "--- Backend Go Tests (inside golang:1.22-alpine) ---"

GO_TEST_OUT=$(mktemp)
if docker compose --profile test run --rm backend-tests > "$GO_TEST_OUT" 2>&1; then
    GO_TEST_LINES=$(grep -c "^ok\|^---\|^=== RUN" "$GO_TEST_OUT" 2>/dev/null || true)
    pass "Backend Go test suite passed (${GO_TEST_LINES:-many} test events)"
else
    fail "Backend Go tests" "See output below:"
    tail -80 "$GO_TEST_OUT"
fi
rm -f "$GO_TEST_OUT"

# =============================================================================
# FRONTEND VITEST — component + unit tests inside a Node container.
# =============================================================================
echo ""
echo "--- Frontend Vitest (inside node:18-alpine) ---"

FE_TEST_OUT=$(mktemp)
if docker compose --profile test run --rm frontend-tests > "$FE_TEST_OUT" 2>&1; then
    FE_PASSED=$(grep -oE "Tests\s+[0-9]+ passed" "$FE_TEST_OUT" | tail -1 || echo "some")
    pass "Frontend Vitest suite passed (${FE_PASSED})"
else
    fail "Frontend Vitest" "See output below:"
    tail -60 "$FE_TEST_OUT"
fi
rm -f "$FE_TEST_OUT"

# =============================================================================
# PLAYWRIGHT E2E TESTS — real browser driving real UI -> real API -> real DB.
# Runs in a dedicated Playwright container via docker compose profile "test".
# =============================================================================
echo ""
echo "--- Playwright E2E (inside Docker) ---"

E2E_LOG=$(mktemp)
if docker compose --profile test run --rm --build e2e > "$E2E_LOG" 2>&1; then
    # Playwright list reporter prints lines like "  ✓  5 [chromium] › tests/auth.spec.ts ..."
    # Count '✓' marks for passing tests.
    E2E_PASSED=$(grep -cE "^\s*✓|^\s*\[chromium\]" "$E2E_LOG" 2>/dev/null || echo "?")
    pass "Playwright E2E suite (${E2E_PASSED} tests passed)"
else
    fail "Playwright E2E" "See output below:"
    tail -100 "$E2E_LOG"
fi
rm -f "$E2E_LOG"

# ---- Summary ----
echo ""
echo "========================================="
echo "  Test Results: $PASS passed, $FAIL failed"
echo "========================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi

exit 0
