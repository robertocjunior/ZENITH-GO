# Zenith-Go Deployment Guide

This guide provides detailed instructions for deploying the Zenith-Go application and its components in various scenarios using Docker Compose.

## Prerequisites

Before you begin, ensure you have the following installed:

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

---

## Core Configuration

### Environment Variables (`.env` file)

The application is configured using environment variables, which can be placed in a `.env` file in the project root. Create this file by copying the example below.

**`.env.example`**
```env
# Sankhya API Credentials (Required)
SANKHYA_API_URL="https://api.sankhya.com.br"
SANKHYA_TRANSACTION_URL="https://api.sankhya.com.br/trans"
SANKHYA_APPKEY="your_sankhya_app_key"
SANKHYA_TOKEN="your_sankhya_token"
SANKHYA_USERNAME="your_sankhya_username"
SANKHYA_PASSWORD="your_sankhya_password"

# Application Security (Required)
JWT_SECRET="a_very_secret_key_for_jwt_signing"

# Redis Connection (Required for Docker setup)
REDIS_ADDR="redis-store:6379"
REDIS_PASSWORD="a_strong_password_for_redis"
REDIS_DB=0

# Log Configuration (Optional)
LOG_MAX_SIZE_MB=100
LOG_MAX_AGE_DAYS=7
```

---

## Deployment Scenarios

Here are several common deployment scenarios, from production to local development.

### Scenario 1: Production Deployment (Full Stack)

This is the standard method for running the entire application stack in a production-like environment. It uses the primary `docker-compose.yml` file to build and run all services.

**`docker-compose.yml` (no changes needed)**
This file is already configured to run two API instances, Redis, and an NGINX load balancer.

**Steps:**
1.  Create a `.env` file with your production credentials.
2.  Run the following command to build and start all services in detached mode:
    ```bash
    docker-compose up -d --build
    ```
The API will be accessible through the NGINX load balancer on port `80`.

### Scenario 2: Local Development with Live Reload

This setup is ideal for development. It uses a custom Dockerfile (`Dockerfile.dev`) and a dedicated compose file to mount your local source code into the container. Changes to your Go files will trigger an automatic rebuild and restart of the application.

We have already created the necessary `Dockerfile.dev` and `.air.toml` files for you.

**`docker-compose.dev.yml`**
Create the following `docker-compose.dev.yml` file in your project root:
```yaml
version: '3.8'

services:
  zenith-api-dev:
    build:
      context: .
      dockerfile: Dockerfile.dev
    restart: on-failure
    environment:
      - REDIS_ADDR=redis-store:6379
    env_file:
      - .env
    volumes:
      # Mount local source code into the container
      - .:/app
    ports:
      # Expose API port directly for debugging
      - "8080:8080"
    depends_on:
      - redis-store

  redis-store:
    image: redis:alpine
    container_name: zenith-redis-dev
    restart: always
    command: redis-server --save 60 1 --loglevel warning --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data_dev:/data

volumes:
  redis_data_dev:
```

**Steps:**
1.  Ensure your `.env` file is present.
2.  Run the following command:
    ```bash
    docker-compose -f docker-compose.dev.yml up -d
    ```
The API will be directly accessible on `http://localhost:8080`. Any changes to files in the `cmd` or `internal` directories will automatically restart the service.

### Scenario 3: Connecting to an External Redis

If you use a managed Redis service (e.g., AWS ElastiCache, Google MemoryStore), you can run the application without the local Redis container.

**`docker-compose.external-redis.yml`**
Create the following `docker-compose.external-redis.yml` file:
```yaml
version: '3.8'

services:
  zenith-api-1:
    build:
      context: .
      dockerfile: Dockerfile
    restart: always
    env_file:
      - .env
    depends_on: {} # No local redis dependency

  zenith-api-2:
    build:
      context: .
      dockerfile: Dockerfile
    restart: always
    env_file:
      - .env
    depends_on: {} # No local redis dependency

  loadbalancer:
    image: nginx:alpine
    container_name: zenith-lb
    restart: always
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
    ports:
      - "80:80"
    depends_on:
      - zenith-api-1
      - zenith-api-2
```

**Steps:**
1.  Update your `.env` file to point to your external Redis instance:
    ```env
    REDIS_ADDR="your-external-redis-host:6379"
    REDIS_PASSWORD="your-external-redis-password"
    REDIS_DB=0
    ```
2.  Run the following command:
    ```bash
    docker-compose -f docker-compose.external-redis.yml up -d
    ```

### Scenario 4: Running without NGINX (Direct API Access)

For testing or if you are using a different load balancing solution, you can run the API nodes and expose their ports directly.

**`docker-compose.no-nginx.yml`**
Create the following `docker-compose.no-nginx.yml` file:
```yaml
version: '3.8'

services:
  zenith-api-1:
    build: .
    restart: always
    env_file: .env
    ports:
      - "8081:8080" # Map host 8081 to container 8080
    depends_on:
      - redis-store

  zenith-api-2:
    build: .
    restart: always
    env_file: .env
    ports:
      - "8082:8080" # Map host 8082 to container 8080
    depends_on:
      - redis-store

  redis-store:
    image: redis:alpine
    container_name: zenith-redis
    restart: always
    command: redis-server --save 60 1 --loglevel warning --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data

volumes:
  redis_data:
```

**Steps:**
1.  Ensure your `.env` file is present.
2.  Run the following command:
    ```bash
    docker-compose -f docker-compose.no-nginx.yml up -d
    ```
You can now access the two API instances directly:
-   **API 1:** `http://localhost:8081`
-   **API 2:** `http://localhost:8082`

---

## Stopping Services

To stop all services and remove the containers for a specific setup, use the `-f` flag with the `down` command.

```bash
# Stop the production stack
docker-compose down

# Stop the development stack
docker-compose -f docker-compose.dev.yml down

# Stop the external-redis stack
docker-compose -f docker-compose.external-redis.yml down

# Stop the no-nginx stack
docker-compose -f docker-compose.no-nginx.yml down
```