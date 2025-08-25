package gateway

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"stox-gateway/internal/config"
)

type MockECommerceClient struct {
	config *config.MockEcommerceConfig
	client *http.Client
}

type ProductResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Price       int    `json:"price"`
		CategoryID  string `json:"categoryId"`
	} `json:"data"`
}

type SingleProductResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Price       int    `json:"price"`
		CategoryID  string `json:"categoryId"`
	} `json:"data"`
}

type CreateProductRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Price       int    `json:"price"`
	CategoryID  string `json:"categoryId"`
}

type UpdateProductRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	CategoryID  string  `json:"categoryId"`
	Stock       int     `json:"stock,omitempty"`
}

type CreateProductResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID          string `json:"id"`
		SellerID    string `json:"sellerId"`
		SellerName  string `json:"sellerName"`
		CategoryID  string `json:"categoryId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Price       int    `json:"price"`
		Stock       int    `json:"stock"`
		Status      int    `json:"status"`
		CreatedAt   string `json:"createdAt"`
		UpdatedAt   string `json:"updatedAt"`
		IsActive    bool   `json:"isActive"`
	} `json:"data"`
}

type DeleteProductResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type UpdateProductResponse struct {
	ID           string    `json:"id"`
	SellerID     string    `json:"sellerId"`
	SellerName   string    `json:"sellerName"`
	CategoryID   string    `json:"categoryId"`
	CategoryName string    `json:"categoryName"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Price        float64   `json:"price"`
	Stock        int       `json:"stock"`
	Status       int       `json:"status"`
	CreatedAt    string    `json:"createdAt"`
	UpdatedAt    string    `json:"updatedAt"`
	IsActive     bool      `json:"isActive"`
}

func NewMockECommerceClient(cfg *config.MockEcommerceConfig) *MockECommerceClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	
	client := &http.Client{
		Timeout:   time.Second * 30,
		Transport: tr,
	}

	return &MockECommerceClient{
		config: cfg,
		client: client,
	}
}

func (c *MockECommerceClient) GetProducts(apiKey string) (*ProductResponse, error) {
	req, err := http.NewRequest("GET", c.config.BaseURL+"/Product/my-products", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *MockECommerceClient) GetProductByID(apiKey, productID string) (*SingleProductResponse, error) {
	req, err := http.NewRequest("GET", c.config.BaseURL+"/Product/"+productID, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result SingleProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *MockECommerceClient) CreateProduct(apiKey string, product CreateProductRequest) (*CreateProductResponse, error) {
	reqBody, err := json.Marshal(product)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.config.BaseURL+"/Product", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result CreateProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *MockECommerceClient) UpdateProduct(apiKey string, productID string, product UpdateProductRequest) (*UpdateProductResponse, error) {
	reqBody, err := json.Marshal(product)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", c.config.BaseURL+"/Product/"+productID, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result UpdateProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *MockECommerceClient) DeleteProduct(apiKey, productID string) (*DeleteProductResponse, error) {
	req, err := http.NewRequest("DELETE", c.config.BaseURL+"/Product/"+productID, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result DeleteProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
