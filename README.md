# <h1 align="center">ZENITH-GO (WMS Backend API)</h1>

<p align="center">
<img alt="Zenith WMS Logo" src="./docs/zenith.svg">
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-Available-2496ED?style=for-the-badge&logo=docker&logoColor=white">
  <img alt="CI/CD" src="https://img.shields.io/badge/GitHub%20Actions-Hybrid%20Pipeline-2088FF?style=for-the-badge&logo=github-actions&logoColor=white">
  <img alt="Sankhya Integration" src="https://img.shields.io/badge/Sankhya-Integrated-52cc63?style=for-the-badge">
</p>

<p align="center">
  <a href="#about-the-project">About</a> •
  <a href="#key-features">Key Features</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#deployment--cicd">Deployment & CI/CD</a> •
  <a href="#api-endpoints">API Endpoints</a> •
  <a href="#license">License</a>
</p>

## About The Project

**ZENITH-GO** is the high-performance evolution of the Zenith WMS backend system. Completely rewritten in Go (Golang), this API serves as the brain for warehouse operations, offering robust, secure, and extremely fast communication with the Sankhya ERP.

This project focuses on processing efficiency, transaction security (Server-Side Validation), and structured logs for auditing, replacing the old Node.js backend for high-demand scenarios.

## Key Features

🚀 **Core Functionality**
- **Hybrid Authentication**: Supports both end-user (Mobile) and Service Account (System) logins simultaneously.
- **Intelligent Search**: Find products by code, description, barcode, or address.
- **Optimized Picking**: Automatic suggestion of picking locations for replenishment.
- **Real-Time History**: Query daily movements, consolidating data from multiple tables (`AD_BXAEND`, `AD_HISTENDAPP`).

🔒 **Security and Authentication**
- **Session Management**: In-memory session control with Sliding Expiration and JWT.
- **Device Validation**: Automatic blocking of unauthorized devices via the `AD_DISPAUT` table.
- **Server-Side Validation**: Business rules (like Picking permission) are validated on the server, ignoring client-side manipulated data.

⚡ **Transactions (ERP Integration)**
- **Transaction Engine**: Executes inventory write-offs, transfers, picking replenishment, and stock adjustments.
- **Sanitization and Security**: Protection against SQL Injection and strict type validation.
- **Merge Logic**: Automatic consolidation of items at the destination to prevent stock fragmentation.
- **Auditing**: Detailed logs for each transaction step in a structured JSON format.

## Architecture

The project follows a clean and modular structure:

- `cmd/api`: Application entry point (`main.go`).
- `internal/config`: Management of environment variables.
- `internal/handler`: HTTP layer (Receives requests, validates JSON, responds to the client).
- `internal/sankhya`: Service and HTTP Client layer (Business logic, communication with ERP, SQL queries).
- `internal/auth`: Logic for JWT, Session, and Devices.
- `internal/logger`: Logging system with automatic rotation and JSON format.

## Getting Started

Follow these instructions to get a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

- **Go**: Version 1.23+ (for local development)
- **Docker & Docker Compose**: (for containerized execution)
- **Git**

### Running with Docker (Recommended)

This is the easiest way to run Zenith-Go on any operating system (Windows, Linux, Mac) without needing to install Go locally.

1.  **Clone the repository**
    ```sh
    git clone [https://github.com/robertocjunior/zenith-go.git](https://github.com/robertocjunior/zenith-go.git)
    cd zenith-go
    ```
2.  **Configure Environment**
    Create a `.env` file in the project root. See the [Configuration](#configuration) section for all available variables.

3.  **Run the application**
    In your terminal, execute:
    ```sh
    docker-compose up -d --build
    ```
The system will be running at `http://localhost:8080`.

-   **Logs**: Logs will be automatically saved in the `./logs_host` folder on your local machine (mapped from the container).
-   **Performance**: The `Dockerfile` uses a Multi-stage build with Alpine Linux, resulting in an extremely lightweight and secure final image.

### Running Locally

If you want to modify the code and test without Docker.

1.  **Clone and Install**
    ```sh
    # Clone the repository
    git clone [https://github.com/robertocjunior/zenith-go.git](https://github.com/robertocjunior/zenith-go.git)
    cd zenith-go

    # Download dependencies
    go mod download
    ```

2.  **Configure Environment**
    Ensure you have a configured `.env` file in the project root.

3.  **Run the application**
    ```sh
    go run cmd/api/main.go
    ```

## Configuration

To run the system, create a `.env` file in the project root. Below are all the available settings.

| Variable | Required | Description | Default |
| :--- | :---: | :--- | :--- |
| **SANKHYA_API_URL** | Yes | Base URL for the Sankhya API (e.g., `https://api.sankhya.com.br`). | |
| **SANKHYA_TRANSACTION_URL** | Yes | URL for executing services/transactions (MGE). | |
| **SANKHYA_APPKEY** | Yes | Application key provided by Sankhya. | |
| **SANKHYA_TOKEN** | Yes | Application token. | |
| **SANKHYA_USERNAME** | Yes | Service user (admin or integrator) in Sankhya. | |
| **SANKHYA_PASSWORD** | Yes | Password for the service user. | |
| **JWT_SECRET** | Yes | Long and random secret key to sign session tokens. | |
| **PORT** | No | Server port. | `8080` |
| **LOG_MAX_SIZE_MB** | No | Maximum size of a log file before rotation (in MB). | `100` |
| **LOG_MAX_AGE_DAYS** | No | Days to keep log files (0 = keep forever/backup based). | `0` |
| **REDIS_ADDR** | No | Address of Redis instance (e.g., `redis-store:6379`). | |
| **REDIS_PASSWORD** | No | Password for Redis authentication. | |
| **EMAIL_NOTIFICATIONS_ENABLED** | No | Enable/Disable email alerts (`true`/`false`). | `false` |
| **SMTP_HOST** | If enabled | SMTP Server Address (e.g., `smtp.office365.com`). | |
| **SMTP_USER** | If enabled | Email account for sending notifications. | |
| **SMTP_PASS** | If enabled | Email password. | |

## Deployment & CI/CD

This project uses a **Hybrid CI/CD Pipeline** via GitHub Actions to ensure zero-downtime deployments and automatic rollback.

### Workflow Overview
1.  **Build (Cloud)**: When code is pushed to `main`, GitHub Actions builds the Docker image and pushes it to **GitHub Container Registry (GHCR)**.
2.  **Deploy (On-Premise)**: A **Self-Hosted Runner** installed on the physical server detects the completion of the build.
3.  **Secret Injection**: Production credentials (`.env`) are securely injected from GitHub Secrets directly into the server during deployment.
4.  **Health Check & Rollback**:
    * The runner updates the containers (`docker compose up -d`).
    * It waits 30 seconds and tests the public health endpoint (`/apiv1/health`).
    * **Success**: Old images are pruned.
    * **Failure**: The system **automatically reverts** to the previous Docker image ID.

### Infrastructure
* **Container Registry**: GHCR (Private)
* **Orchestration**: Docker Compose
* **Reverse Proxy**: NGINX (Load Balancing & SSL)

## API Endpoints

The API exposes the following endpoints, all prefixed with `/apiv1`.

### Authentication
- `POST /apiv1/login`: Logs the user in (validates in ERP, checks device, generates JWT).
- `POST /apiv1/logout`: Ends the session.
- `GET /apiv1/permissions`: Returns the permissions of the logged-in user (WMS).

### Products & Stock
- `POST /apiv1/search-items`: Searches for products in stock with filters.
- `POST /apiv1/get-item-details`: Gets specific details of an item (including batch/expiry).
- `POST /apiv1/get-picking-locations`: Finds alternative picking locations for a product.
- `POST /apiv1/get-history`: Returns the user's movement history for the day.

### Transactions
- `POST /apiv1/execute-transaction`: Unified endpoint for performing operations.
    - **Supported Types**: `baixa`, `transferencia`, `picking`, `correcao`.
    - **Required Headers**: `Authorization` (Bearer) and `Snkjsessionid` (Sankhya Cookie).

### [📚 API Reference & Examples](./docs/API_Reference_and_Examples.md)

## License

This software is proprietary and confidential. Copyright © 2025 Roberto Casali Junior. All rights reserved.
