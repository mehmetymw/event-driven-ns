#!/bin/bash
BASE="http://localhost:8080/api/v1"
echo "━━━ Email Notification ━━━"

RESP=$(curl -s -X POST "$BASE/notifications" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "recipient": "user@example.com",
    "content": "Email test notification",
    "priority": "normal"
  }')
ID=$(echo "$RESP" | jq -r '.id')
echo "$RESP" | jq '{id,channel,recipient,status}'

sleep 2
echo -e "\n━━━ Status ━━━"
curl -s "$BASE/notifications/$ID" | jq '{id,status,sent_at,content}'
