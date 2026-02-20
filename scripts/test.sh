#!/bin/bash
set -e
BASE="http://localhost:8080/api/v1"
GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
hr() { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }

hr "1. Create Template (Email)"
TMPL=$(curl -s -X POST "$BASE/templates" \
  -H "Content-Type: application/json" \
  -d '{"name":"welcome","channel":"email","body":"Hello {{.Name}}, welcome!"}')
TMPL_ID=$(echo "$TMPL" | jq -r '.id')
echo "$TMPL" | jq .
echo -e "${GREEN}Template ID: $TMPL_ID${NC}"

hr "2. Send Notification with Template"
NOTIF=$(curl -s -X POST "$BASE/notifications" \
  -H "Content-Type: application/json" \
  -d "{
    \"channel\": \"email\",
    \"recipient\": \"test@example.com\",
    \"content\": \"fallback\",
    \"priority\": \"high\",
    \"template_id\": \"$TMPL_ID\",
    \"template_variables\": {\"Name\": \"Mehmet\"}
  }")
NOTIF_ID=$(echo "$NOTIF" | jq -r '.id')
echo "$NOTIF" | jq .
echo -e "${GREEN}Notification ID: $NOTIF_ID${NC}"

hr "3. Notification Status"
sleep 2
curl -s "$BASE/notifications/$NOTIF_ID" | jq '{id,channel,status,content,sent_at}'

hr "4. Send Batch (3 notifications)"
BATCH=$(curl -s -X POST "$BASE/notifications/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"channel":"email","recipient":"a@test.com","content":"Batch email 1","priority":"normal"},
      {"channel":"sms","recipient":"+905530050594","content":"Batch SMS","priority":"high"},
      {"channel":"push","recipient":"device-token-abc","content":"Batch push","priority":"low"}
    ]
  }')
BATCH_ID=$(echo "$BATCH" | jq -r '.batch.id')
echo "$BATCH" | jq '.batch'
echo -e "${GREEN}Batch ID: $BATCH_ID${NC}"

hr "5. Batch Status"
sleep 2
curl -s "$BASE/batches/$BATCH_ID" | jq .

hr "6. List All Notifications"
curl -s "$BASE/notifications?page_size=10" | jq '.data[] | {id,channel,status,recipient}'

hr "7. Metrics"
curl -s "$BASE/metrics" | jq .

echo -e "\n${GREEN}All tests completed!${NC}"
