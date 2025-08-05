#!/bin/bash

# STOX Gateway End-to-End Test Scenario
# Bu script sistemin tüm bileşenlerini test eder

set -e

echo "🧪 STOX Gateway End-to-End Test Başlatılıyor..."
echo "=========================================="

BASE_URL="http://localhost:8080"
TEST_EMAIL="test-$(date +%s)@test.com"
TEST_PASSWORD="MySecureP@ssw0rd2025!"
TEST_IMAGE="test-image.jpg"

# Test resmi kontrol et
check_test_image() {
    if [ ! -f "$TEST_IMAGE" ]; then
        echo "⚠️ Test resmi bulunamadı: $TEST_IMAGE"
        echo "📸 test3-sandalye.jpeg dosyasını kopyalıyoruz..."
        if [ -f "../../test3-sandalye.jpeg" ]; then
            cp "../../test3-sandalye.jpeg" "$TEST_IMAGE"
            echo "✅ Test resmi kopyalandı: $TEST_IMAGE"
        else
            echo "❌ test3-sandalye.jpeg dosyası bulunamadı!"
            exit 1
        fi
    else
        echo "✅ Test resmi mevcut: $TEST_IMAGE"
    fi
}

# 1. Health Check
test_health() {
    echo "🏥 Health Check..."
    curl -sf "$BASE_URL/health" >/dev/null || {
        echo "❌ Gateway çalışmıyor! docker-compose up -d çalıştırın"
        exit 1
    }
    echo "✅ Gateway çalışıyor"
}

# 2. Kullanıcı Kaydı
test_register() {
    echo "👤 Kullanıcı kaydı test ediliyor..."
    REGISTER_RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\",
            \"firstName\": \"Test\",
            \"lastName\": \"User\",
            \"role\": \"user\"
        }")
    
    echo "Register Response: $REGISTER_RESPONSE"
    
    if echo "$REGISTER_RESPONSE" | grep -q "success.*true"; then
        echo "✅ Kullanıcı kaydı başarılı"
    else
        echo "⚠️ Kullanıcı kaydı başarısız (muhtemelen auth service çalışmıyor)"
    fi
}

# 3. Giriş Yap ve Token Al
test_login() {
    echo "🔐 Giriş testi..."
    LOGIN_RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")
    
    echo "Login Response: $LOGIN_RESPONSE"
    
    # Token'ı çıkar (jq varsa)
    if command -v jq &> /dev/null; then
        TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.tokenData.accessToken // empty')
    else
        # jq yoksa basit grep
        TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"accessToken":"[^"]*"' | cut -d'"' -f4)
    fi
    
    if [ -n "$TOKEN" ]; then
        echo "✅ Giriş başarılı, Token alındı"
        echo "Token: ${TOKEN:0:20}..."
    else
        echo "❌ Token alınamadı"
        return 1
    fi
}

# 4. Resim Upload Test
test_image_upload() {
    echo "📤 Resim upload testi..."
    
    if [ -z "$TOKEN" ]; then
        echo "❌ Token yok, upload test edilemiyor"
        return 1
    fi
    
    UPLOAD_RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v1/images/upload" \
        -H "Authorization: Bearer $TOKEN" \
        -F "image=@$TEST_IMAGE" \
        -F "productName=Test Product")
    
    echo "Upload Response: $UPLOAD_RESPONSE"
    
    if echo "$UPLOAD_RESPONSE" | grep -q "success.*true"; then
        echo "✅ Resim upload başarılı"
        
        # CloudFront URL kontrol et
        if echo "$UPLOAD_RESPONSE" | grep -q "cloudfront"; then
            echo "✅ CloudFront URL oluşturuldu"
        fi
    else
        echo "❌ Resim upload başarısız"
    fi
}

# 5. Resim Listeleme Test
test_image_list() {
    echo "📋 Resim listeleme testi..."
    
    if [ -z "$TOKEN" ]; then
        echo "❌ Token yok, list test edilemiyor"
        return 1
    fi
    
    LIST_RESPONSE=$(curl -sf -X GET "$BASE_URL/api/v1/images/list" \
        -H "Authorization: Bearer $TOKEN")
    
    echo "List Response: $LIST_RESPONSE"
    
    if echo "$LIST_RESPONSE" | grep -q "success.*true"; then
        echo "✅ Resim listeleme başarılı"
    else
        echo "❌ Resim listeleme başarısız"
    fi
}

# 6. AWS S3 Kontrolü
test_s3_bucket() {
    echo "🪣 S3 bucket kontrolü..."

    if aws s3 ls ${AWS_S3_BUCKET_NAME} &>/dev/null; then
        echo "✅ S3 bucket erişilebilir"
        
        # Kullanıcı klasörü var mı kontrol et
        if aws s3 ls s3://${AWS_S3_BUCKET_NAME}/users/ &>/dev/null; then
            echo "✅ Users klasörü mevcut"
        fi
    else
        echo "❌ S3 bucket erişilemiyor"
    fi
}

# 7. CloudFront Kontrolü
test_cloudfront() {
    echo "🌐 CloudFront kontrolü..."
    
    # Get CloudFront domain from .env if available
    if [ -f "../.env" ]; then
        source ../.env
        CF_URL="https://${AWS_CLOUDFRONT_DOMAIN_NAME}"
    else
        CF_URL="Error! CloudFront domain name bulunamadı. Lütfen .env dosyasını kontrol edin."
    fi
    
    # Test CloudFront connectivity (403 is expected for root path)
    response=$(curl -s --head --max-time 10 "$CF_URL" 2>/dev/null)
    if echo "$response" | grep -qi "cloudfront"; then
        echo "✅ CloudFront erişilebilir"
        echo "   Domain: $(echo $CF_URL | sed 's|https://||')"
    else
        echo "⚠️ CloudFront erişilemiyor veya timeout"
        echo "   Test URL: $CF_URL"
    fi
}

# Ana Test Fonksiyonu
run_tests() {
    check_test_image
    test_health
    test_s3_bucket
    test_cloudfront
    test_register
    test_login
    test_image_upload
    test_image_list
}

# Cleanup
cleanup() {
    echo "🧹 Test dosyası temizleniyor..."
    [ -f "$TEST_IMAGE" ] && rm -f "$TEST_IMAGE"
}

# Test çalıştır
echo "🚀 Testler başlatılıyor..."
run_tests

echo ""
echo "=========================================="
echo "🏁 Test Tamamlandı!"
echo "=========================================="

cleanup

# Service durumları
echo ""
echo "📊 Service Durumları:"
echo "🔄 Gateway: $(curl -sf $BASE_URL/health >/dev/null && echo "✅ Çalışıyor" || echo "❌ Çalışmıyor")"
echo "🔐 Auth Service: $(nc -z localhost 50051 && echo "✅ Port açık" || echo "❌ Port kapalı")"
echo "🖼️ Image Service: $(nc -z localhost 50061 && echo "✅ Port açık" || echo "❌ Port kapalı")"
echo ""
echo "💡 Eksik olan servisleri docker-compose ile başlatın:"
echo "   docker-compose up -d"
