# Stox Gateway - Workflow Orchestration System

This project implements a workflow orchestration system based on the provided sequence diagram, using RabbitMQ for async messaging and WebSocket for real-time updates.

## Architecture Overview

```
Frontend → Gateway → RabbitMQ → Services (Image/AI/SEO)
    ↑         ↓
    └── WebSocket ← Gateway ← gRPC ← Services
```

## Project Structure

```
stox-gateway/
├── cmd/
│   └── api-gateway/
│       └── main.go                 # Main application entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration management
│   ├── events/
│   │   ├── bus.go                  # RabbitMQ event publisher
│   │   └── marshaling.go           # Event marshaling utilities
│   ├── gateway/
│   │   ├── handler.go              # Auth HTTP handlers
│   │   ├── workflow_handler.go     # Workflow HTTP handlers
│   │   ├── middleware.go           # HTTP middleware
│   │   └── router.go               # HTTP routing
│   ├── grpcclients/
│   │   └── auth_client.go          # gRPC client for auth service
│   ├── logger/
│   │   └── logger.go               # Structured logging
│   ├── websocket/
│   │   └── hub.go                  # WebSocket hub for real-time updates
│   └── workflow/
│       ├── models.go               # Workflow data models
│       └── manager.go              # Workflow orchestration logic
├── examples/
│   └── publisher_example.go        # Example of using the event publisher
├── scripts/
│   └── generate.bat                # Protocol buffer generation script
├── config.example.yaml             # Example configuration file
├── go.mod                          # Go module dependencies
├── go.sum                          # Go module checksums
└── Makefile                        # Build and run targets
```

## Key Components

### 1. Workflow System (`internal/workflow/`)

**Models (`models.go`):**
- `Workflow`: Complete workflow state with steps and metadata
- `WorkflowStep`: Individual step in the workflow
- `ProductUploadRequest`: Initial product data structure
- `StepCompletedRequest`: Service completion notification

**Manager (`manager.go`):**
- `CreateWorkflow()`: Creates and starts new workflows
- `CompleteStep()`: Processes step completions from services
- `GetWorkflow()`: Retrieves workflow status

### 2. Event System (`internal/events/`)

**Event Bus (`bus.go`):**
- RabbitMQ publisher with retry logic
- Different event types for workflow stages
- Routing keys for service-specific messaging

**Event Types:**
- `workflow.start`: Initial workflow creation
- `step.image_processing`: Route to Image Service
- `step.ai_enhancement`: Route to AI Service  
- `step.seo_generation`: Route to SEO Service
- `step.completed`: Internal step tracking
- `workflow.completed`: Final workflow notification

### 3. Real-time Updates (`internal/websocket/`)

**WebSocket Hub (`hub.go`):**
- Real-time client connections
- Workflow-specific subscriptions
- Broadcast notifications for step/workflow completion

### 4. HTTP API (`internal/gateway/`)

**Workflow Endpoints:**
- `POST /api/v1/workflow/upload` - Upload product and start workflow
- `GET /api/v1/workflow/status?workflow_id=xxx` - Get workflow status
- `POST /api/v1/workflow/step-completed` - Service completion notification
- `GET /ws?workflow_id=xxx` - WebSocket connection for real-time updates

## Configuration

Copy `config.example.yaml` to `config.yaml` and adjust settings:

```yaml
server:
  port: 8080
  environment: development

rabbitmq:
  url: amqp://guest:guest@localhost:5672/
  exchange: workflow_exchange
  max_retries: 3
  retry_delay: 5s

services:
  auth:
    host: localhost
    port: 50051
  image:
    host: localhost
    port: 8000
  # ... other services
```

## Usage Examples

### 1. Start the Gateway

```bash
# Copy example config
cp config.example.yaml config.yaml

# Install dependencies
go mod tidy

# Run the gateway
make run-dev
```

### 2. Upload Product (Frontend → Gateway)

```bash
curl -X POST http://localhost:8080/api/v1/workflow/upload \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sample Product",
    "description": "A test product",
    "images": ["image1.jpg", "image2.jpg"],
    "category": "Electronics",
    "price": 99.99
  }'
```

Response:
```json
{
  "workflow_id": "uuid-here",
  "status": "processing",
  "message": "Workflow started successfully"
}
```

### 3. Service Completion Notification (Service → Gateway)

```bash
curl -X POST http://localhost:8080/api/v1/workflow/step-completed \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_id": "uuid-here",
    "step_id": "step-uuid",
    "step_type": "image_processing",
    "success": true,
    "result": {
      "processed_images": ["processed1.jpg", "processed2.jpg"],
      "processing_time": "2.5s"
    }
  }'
```

### 4. Real-time Updates (Frontend WebSocket)

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?workflow_id=uuid-here');

ws.onmessage = function(event) {
  const message = JSON.parse(event.data);
  console.log('Workflow update:', message);
  
  switch(message.type) {
    case 'step_completed':
      console.log('Step completed:', message.data.step_id);
      break;
    case 'workflow_completed':
      console.log('Workflow completed!');
      break;
    case 'workflow_failed':
      console.log('Workflow failed:', message.data.error);
      break;
  }
};
```

### 5. Check Workflow Status

```bash
curl "http://localhost:8080/api/v1/workflow/status?workflow_id=uuid-here"
```

## Publisher Example

Run the example to see how events are published:

```bash
go run examples/publisher_example.go
```

This demonstrates:
1. Creating workflows with product data
2. Publishing workflow start events
3. Publishing step progression events
4. Publishing step completion events
5. Publishing workflow completion events

## RabbitMQ Message Routing

The system uses topic exchange with routing keys:

- `workflow.start` → All services can listen for new workflows
- `step.image_processing` → Image Service queue
- `step.ai_enhancement` → AI Service queue
- `step.seo_generation` → SEO Service queue
- `step.completed` → Internal gateway processing
- `workflow.completed` → Notification services
- `workflow.failed` → Error handling services

## Service Implementation Guide

Each microservice should:

1. **Listen to RabbitMQ** for their specific routing key
2. **Process the workflow step** 
3. **Notify Gateway via gRPC/HTTP** when complete

Example service consumer:
```go
// Listen for step.image_processing messages
ch.QueueBind(
    queueName,           // queue name
    "step.image_processing", // routing key  
    "workflow_exchange", // exchange
    false,
    nil,
)

// Process message and notify gateway
stepResult := processImages(workflowData)
notifyGateway(workflowID, stepID, stepResult)
```

## Dependencies

- **RabbitMQ**: Message broker for async communication
- **gorilla/websocket**: WebSocket support for real-time updates
- **gorilla/mux**: HTTP routing
- **go.uber.org/zap**: Structured logging
- **spf13/viper**: Configuration management
- **google/uuid**: UUID generation
- **grpc**: gRPC client support

## Development

```bash
# Build
make build

# Run in development mode
make run-dev

# Clean build artifacts
make clean
```

## Monitoring

- Health check: `GET /health`
- List all workflows: `GET /api/v1/workflow/list`
- WebSocket connections are logged with client tracking
- All RabbitMQ events are logged with correlation IDs
