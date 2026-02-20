#!/bin/bash
BASE="http://localhost:8080/api/v1"
echo "━━━ SMS Notification ━━━"

RESP=$(curl -s -X POST "$BASE/notifications" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "sms",
    "recipient": "+905530050594",
    "content": "SMS test notification",
    "priority": "high"
  }')
ID=$(echo "$RESP" | jq -r '.id')
echo "$RESP" | jq '{id,channel,recipient,status}'

sleep 2
echo -e "\n━━━ Status ━━━"
curl -s "$BASE/notifications/$ID" | jq '{id,status,sent_at,content}'
