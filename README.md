# Stox API Gateway

This is the API Gateway for the Stox platform, responsible for routing requests to the appropriate microservices.

## Development

### Prerequisites
- Go 1.21 or higher
- Docker and Docker Compose (for containerized development)

### Local Development

Build and run locally:
```bash
make build
make run
```

For development mode with hot reloading:
```bash
make run-dev
```

### Docker Development

Build the Docker image:
```bash
make docker-build
```

Run the Docker container:
```bash
make docker-run
```

### Docker Compose

Start all services:
```bash
make docker-compose-up
```

Stop all services:
```bash
make docker-compose-down
```

## Configuration

Configuration can be provided via a `config.yaml` file or environment variables.
When running in Docker, mount your config file or provide environment variables as shown in the `docker-compose.yml` file.

## Project Structure

- `/cmd/api-gateway`: Main application entry point
- `/internal`: Internal packages
  - `/config`: Configuration management
  - `/events`: Event bus implementation
  - `/gateway`: API handlers and routing
  - `/grpcclients`: gRPC client implementations
  - `/logger`: Logging utilities
  - `/proto`: Protocol buffer definitions

## Dockerization

The project includes Docker and Docker Compose configuration for easy deployment:

- `Dockerfile`: Multi-stage build for a small and secure container image
- `docker-compose.yml`: Orchestrates the API gateway and its dependent services
- `.dockerignore`: Excludes unnecessary files from the Docker build context

By default, the Docker container runs on port 8080. This can be changed in the `docker-compose.yml` file.