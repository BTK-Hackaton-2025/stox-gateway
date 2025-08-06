package ecommerce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MockECommerceClient handles integration with MockECommerce system
type MockECommerceClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewMockECommerceClient creates a new client for MockECommerce API
func NewMockECommerceClient(baseURL, apiKey string) *MockECommerceClient {
	return &MockECommerceClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Product represents a product from MockECommerce
type Product struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	IsActive    bool      `json:"isActive"`
	Status      string    `json:"status"`
	CategoryID  string    `json:"categoryId"`
	SellerID    string    `json:"sellerId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Images      []ProductImage `json:"images"`
}

// ProductImage represents a product image
type ProductImage struct {
	ID        string `json:"id"`
	ImageUrl  string `json:"imageUrl"`
	IsPrimary bool   `json:"isPrimary"`
	AltText   string `json:"altText"`
}

// CreateProductRequest represents a product creation request
type CreateProductRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	CategoryID  string  `json:"categoryId"`
	Images      []CreateProductImageRequest `json:"images,omitempty"`
}

// CreateProductImageRequest represents an image in product creation
type CreateProductImageRequest struct {
	ImageUrl  string `json:"imageUrl"`
	IsPrimary bool   `json:"isPrimary"`
	AltText   string `json:"altText"`
}

// Order represents an order from MockECommerce
type Order struct {
	ID          string    `json:"id"`
	ProductID   string    `json:"productId"`
	ProductName string    `json:"productName"`
	CustomerID  string    `json:"customerId"`
	Status      string    `json:"status"`
	OrderDate   time.Time `json:"orderDate"`
	Product     *Product  `json:"product,omitempty"`
}

// ProductStatistics represents seller's product statistics
type ProductStatistics struct {
	TotalProducts       int     `json:"totalProducts"`
	ActiveProducts      int     `json:"activeProducts"`
	InactiveProducts    int     `json:"inactiveProducts"`
	DraftProducts       int     `json:"draftProducts"`
	ActiveStatusProducts int    `json:"activeStatusProducts"`
	BlockedProducts     int     `json:"blockedProducts"`
	TotalValue          float64 `json:"totalValue"`
}

// makeRequest makes an HTTP request with API key authentication
func (c *MockECommerceClient) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	return c.httpClient.Do(req)
}

// GetSellerProducts retrieves all products for the authenticated seller
func (c *MockECommerceClient) GetSellerProducts(page, pageSize int) ([]Product, int, error) {
	endpoint := fmt.Sprintf("/api/v1/external/products?page=%d&pageSize=%d", page, pageSize)
	
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get seller products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var response struct {
		Products   []Product `json:"products"`
		TotalCount int       `json:"totalCount"`
		Page       int       `json:"page"`
		PageSize   int       `json:"pageSize"`
		TotalPages int       `json:"totalPages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Products, response.TotalCount, nil
}

// GetProductByID retrieves a specific product by ID
func (c *MockECommerceClient) GetProductByID(productID string) (*Product, error) {
	endpoint := fmt.Sprintf("/api/v1/external/products/%s", productID)
	
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var product Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &product, nil
}

// CreateProduct creates a new product
func (c *MockECommerceClient) CreateProduct(req CreateProductRequest) (*Product, error) {
	endpoint := "/api/v1/external/products"
	
	resp, err := c.makeRequest("POST", endpoint, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var product Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &product, nil
}

// UpdateProduct updates an existing product
func (c *MockECommerceClient) UpdateProduct(productID string, req CreateProductRequest) (*Product, error) {
	endpoint := fmt.Sprintf("/api/v1/external/products/%s", productID)
	
	resp, err := c.makeRequest("PUT", endpoint, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var product Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &product, nil
}

// DeleteProduct deletes a product
func (c *MockECommerceClient) DeleteProduct(productID string) error {
	endpoint := fmt.Sprintf("/api/v1/external/products/%s", productID)
	
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetProductStatistics retrieves seller's product statistics
func (c *MockECommerceClient) GetProductStatistics() (*ProductStatistics, error) {
	endpoint := "/api/v1/external/products/statistics"
	
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get product statistics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var stats ProductStatistics
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stats, nil
}

// GetSellerOrders retrieves orders for the seller (from seller ID in API key)
func (c *MockECommerceClient) GetSellerOrders(sellerID string) ([]Order, error) {
	endpoint := fmt.Sprintf("/api/v1/order/seller/%s", sellerID)
	
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get seller orders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var response struct {
		Success bool    `json:"success"`
		Data    []Order `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Data, nil
}
