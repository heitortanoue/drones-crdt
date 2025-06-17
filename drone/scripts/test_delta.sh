#!/bin/bash

# Script de teste rápido para verificar formato JSON

set -e

echo "=== Teste de Formato JSON para Delta ==="

# IDs fixos para teste
uuid1="11111111-1111-1111-1111-111111111111"
ts1=$(date +%s)000  # Timestamp em milissegundos

echo "UUID: $uuid1"
echo "Timestamp: $ts1"

# Formato JSON que estamos enviando
json_payload=$(cat <<EOF
{
  "id":        "$uuid1",
  "ttl":       3,
  "sender_id": "drone-2",
  "timestamp": $ts1,
  "data": {
    "id":        "$uuid1",
    "sensor_id": "cross-drone-sensor-A",
    "timestamp": $ts1,
    "value":     87.3,
    "drone_id":  "drone-2"
  }
}
EOF
)

echo
echo "JSON a ser enviado:"
echo "$json_payload"

echo
echo "Verificando se é JSON válido:"
if echo "$json_payload" | jq . >/dev/null 2>&1; then
    echo "✅ JSON é válido"
    echo
    echo "JSON formatado:"
    echo "$json_payload" | jq .
else
    echo "❌ JSON é inválido"
fi

echo
echo "Testando envio para drone-1 (assumindo que está rodando na porta 8080):"
echo "curl -s -X POST http://localhost:8080/delta -H 'Content-Type: application/json' -d '...'"

response=$(curl -s -X POST http://localhost:8080/delta \
     -H "Content-Type: application/json" \
     -d "$json_payload")

echo "Resposta: $response"

if echo "$response" | jq . >/dev/null 2>&1; then
    echo "✅ Resposta é JSON válido"
    echo "$response" | jq .
else
    echo "❌ Resposta não é JSON válido"
fi
