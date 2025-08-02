package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"stox-gateway/internal/config"
	"stox-gateway/internal/events"
	"stox-gateway/internal/workflow"

	"go.uber.org/zap"
)

func main() {
	// Example demonstrating how to use the RabbitMQ publisher
	
	// Create a logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Example RabbitMQ configuration
	cfg := &config.RabbitMQConfig{
		URL:        "amqp://guest:guest@localhost:5672/",
		Exchange:   "workflow_exchange",
		RoutingKey: "workflow",
		MaxRetries: 3,
		RetryDelay: 5 * time.Second,
	}

	// Create event bus
	eventBus, err := events.NewEventBus(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create event bus: %v", err)
	}
	defer eventBus.Close()

	// Example 1: Create and start a workflow
	fmt.Println("=== Example 1: Starting a Workflow ===")
	
	productData := &workflow.ProductUploadRequest{
		Name:        "Sample Product",
		Description: "A sample product for testing",
		Images:      []string{"image1.jpg", "image2.jpg"},
		Category:    "Electronics",
		Price:       99.99,
		Metadata: map[string]interface{}{
			"brand":  "TestBrand",
			"weight": "1.2kg",
		},
	}

	// Create workflow
	wf := workflow.NewWorkflow("product_123", productData)
	
	// Print workflow details
	workflowJSON, _ := json.MarshalIndent(wf, "", "  ")
	fmt.Printf("Created workflow:\n%s\n\n", workflowJSON)

	// Publish workflow start event
	ctx := context.Background()
	if err := eventBus.PublishWorkflowStart(ctx, wf); err != nil {
		log.Fatalf("Failed to publish workflow start: %v", err)
	}
	fmt.Println("✅ Published workflow start event")

	// Example 2: Publish next step event
	fmt.Println("\n=== Example 2: Starting Image Processing Step ===")
	
	currentStep := wf.GetCurrentStep()
	if currentStep != nil {
		// Start the step
		wf.StartStep(currentStep.ID)
		
		if err := eventBus.PublishNextStep(ctx, wf, currentStep); err != nil {
			log.Fatalf("Failed to publish next step: %v", err)
		}
		fmt.Printf("✅ Published next step event for: %s (%s)\n", currentStep.Type, currentStep.ServiceName)
	}

	// Example 3: Simulate step completion
	fmt.Println("\n=== Example 3: Completing Image Processing Step ===")
	
	// Simulate successful step completion
	result := map[string]interface{}{
		"processed_images": []string{"processed_image1.jpg", "processed_image2.jpg"},
		"processing_time":  "2.5s",
		"status":          "success",
	}

	wf.MarkStepCompleted(currentStep.ID, result)
	
	if err := eventBus.PublishStepCompleted(ctx, wf, currentStep.ID, result); err != nil {
		log.Fatalf("Failed to publish step completed: %v", err)
	}
	fmt.Println("✅ Published step completed event")

	// Example 4: Start next step (AI Enhancement)
	fmt.Println("\n=== Example 4: Starting AI Enhancement Step ===")
	
	nextStep := wf.GetCurrentStep()
	if nextStep != nil {
		wf.StartStep(nextStep.ID)
		
		if err := eventBus.PublishNextStep(ctx, wf, nextStep); err != nil {
			log.Fatalf("Failed to publish next step: %v", err)
		}
		fmt.Printf("✅ Published next step event for: %s (%s)\n", nextStep.Type, nextStep.ServiceName)
	}

	// Example 5: Complete AI Enhancement and start SEO
	fmt.Println("\n=== Example 5: Completing AI Enhancement and Starting SEO ===")
	
	// Complete AI step
	aiResult := map[string]interface{}{
		"enhanced_description": "AI-enhanced product description with better keywords",
		"suggested_tags":       []string{"electronics", "high-quality", "durable"},
		"ai_score":            0.92,
	}

	wf.MarkStepCompleted(nextStep.ID, aiResult)
	
	if err := eventBus.PublishStepCompleted(ctx, wf, nextStep.ID, aiResult); err != nil {
		log.Fatalf("Failed to publish AI step completed: %v", err)
	}
	fmt.Println("✅ Published AI step completed event")

	// Start SEO step
	seoStep := wf.GetCurrentStep()
	if seoStep != nil {
		wf.StartStep(seoStep.ID)
		
		if err := eventBus.PublishNextStep(ctx, wf, seoStep); err != nil {
			log.Fatalf("Failed to publish SEO step: %v", err)
		}
		fmt.Printf("✅ Published next step event for: %s (%s)\n", seoStep.Type, seoStep.ServiceName)
	}

	// Example 6: Complete final step and workflow
	fmt.Println("\n=== Example 6: Completing SEO Step and Workflow ===")
	
	seoResult := map[string]interface{}{
		"seo_title":       "High-Quality Sample Product - Best Electronics 2024",
		"meta_description": "Discover our premium sample product with cutting-edge technology and unmatched durability.",
		"keywords":        []string{"sample product", "electronics", "high quality", "durable"},
		"seo_score":       95,
	}

	wf.MarkStepCompleted(seoStep.ID, seoResult)
	
	if err := eventBus.PublishStepCompleted(ctx, wf, seoStep.ID, seoResult); err != nil {
		log.Fatalf("Failed to publish SEO step completed: %v", err)
	}
	fmt.Println("✅ Published SEO step completed event")

	// Workflow should be completed now
	if wf.Status == workflow.StatusCompleted {
		if err := eventBus.PublishWorkflowCompleted(ctx, wf); err != nil {
			log.Fatalf("Failed to publish workflow completed: %v", err)
		}
		fmt.Println("🎉 Published workflow completed event")
	}

	// Print final workflow state
	fmt.Println("\n=== Final Workflow State ===")
	finalWorkflowJSON, _ := json.MarshalIndent(wf, "", "  ")
	fmt.Printf("Final workflow state:\n%s\n", finalWorkflowJSON)

	fmt.Println("\n=== Publisher Example Completed Successfully! ===")
	fmt.Println("This example demonstrates:")
	fmt.Println("1. Creating a workflow with product data")
	fmt.Println("2. Publishing workflow start events")
	fmt.Println("3. Publishing step progression events")
	fmt.Println("4. Publishing step completion events")
	fmt.Println("5. Publishing workflow completion events")
	fmt.Println("\nEach message is published to RabbitMQ with appropriate routing keys:")
	fmt.Println("- workflow.start -> Initial workflow creation")
	fmt.Println("- step.image_processing -> Route to Image Service")
	fmt.Println("- step.ai_enhancement -> Route to AI Service") 
	fmt.Println("- step.seo_generation -> Route to SEO Service")
	fmt.Println("- step.completed -> Internal step tracking")
	fmt.Println("- workflow.completed -> Final workflow notification")
}
