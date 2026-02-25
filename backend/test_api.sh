#!/bin/bash

# Скрипт для тестирования API endpoints

BASE_URL="http://localhost:8080/api"

echo "================================"
echo "Testing Invoice Backend API"
echo "================================"
echo ""

# Цвета для вывода
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для тестирования endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo -e "${BLUE}Testing: ${description}${NC}"
    echo "Request: ${method} ${BASE_URL}${endpoint}"
    
    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X ${method} "${BASE_URL}${endpoint}")
    else
        response=$(curl -s -w "\n%{http_code}" -X ${method} \
            -H "Content-Type: application/json" \
            -d "${data}" \
            "${BASE_URL}${endpoint}")
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo -e "${GREEN}✓ Success (HTTP ${http_code})${NC}"
        echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"
    else
        echo -e "${RED}✗ Failed (HTTP ${http_code})${NC}"
        echo "$body"
    fi
    echo ""
}

# 1. Тест получения списка контрагентов
test_endpoint "GET" "/customers" "" "Get all customers"

# 2. Тест поиска контрагентов
test_endpoint "GET" "/customers?search=Ромашка" "" "Search customers by name"

# 3. Создание новой услуги
echo -e "${BLUE}Creating test service...${NC}"
service_response=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d '{"name":"Тестовая услуга","price":1000.00}' \
    "${BASE_URL}/services")
service_id=$(echo "$service_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)

if [ ! -z "$service_id" ]; then
    echo -e "${GREEN}✓ Service created: ${service_id}${NC}"
    echo ""
    
    # 4. Получаем первого контрагента для создания счета
    echo -e "${BLUE}Getting first customer...${NC}"
    customer_response=$(curl -s "${BASE_URL}/customers")
    customer_id=$(echo "$customer_response" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['data'][0]['id'] if data['data'] else '')" 2>/dev/null)
    
    if [ ! -z "$customer_id" ]; then
        echo -e "${GREEN}✓ Customer ID: ${customer_id}${NC}"
        echo ""
        
        # 5. Создаем счет
        invoice_data="{\"customer_id\":\"${customer_id}\",\"contract_number\":\"Основной\",\"number\":\"TEST-001\",\"date\":\"13.01.2026\",\"service_ids\":[\"${service_id}\"]}"
        test_endpoint "POST" "/invoices" "$invoice_data" "Create invoice"
        
        # 6. Получаем созданный счет
        invoice_response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "$invoice_data" \
            "${BASE_URL}/invoices")
        invoice_id=$(echo "$invoice_response" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
        
        if [ ! -z "$invoice_id" ]; then
            # 7. Получаем счет с услугами
            test_endpoint "GET" "/invoices/${invoice_id}/services" "" "Get invoice with services"
            
            # 8. Получаем все счета контрагента
            test_endpoint "GET" "/invoices?customer_id=${customer_id}" "" "Get invoices by customer"
        fi
    else
        echo -e "${RED}✗ No customers found. Please add test data first.${NC}"
        echo "Run: psql -d invoices_db -f migrations/002_test_data.sql"
        echo ""
    fi
else
    echo -e "${RED}✗ Failed to create service${NC}"
    echo ""
fi

echo "================================"
echo "Testing complete!"
echo "================================"
