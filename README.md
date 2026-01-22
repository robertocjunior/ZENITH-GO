# ZENITH-GO (WMS Backend API)

<div align="center">
  <img alt="Zenith WMS Logo" src="./docs/zenith.svg">
  <br>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white">
  <img alt="Sankhya" src="https://img.shields.io/badge/ERP-Sankhya_Integrated-52cc63">
</div>

## 📋 Sobre o Projeto
O **ZENITH-GO** é a evolução de alta performance do backend WMS Zenith. Reescrevemos a API em **Go (Golang)** para garantir latência mínima, segurança robusta e integração nativa com o ERP Sankhya.

Foca em operações críticas de armazém: **Separação (Picking)**, **Conferência**, **Transferência** e **Auditoria**.

## 📚 Documentação
A documentação detalhada foi dividida para facilitar a manutenção:

- **[Arquitetura e Design](./docs/ARCHITECTURE.md)**: Entenda a estrutura de pastas (`cmd`, `internal`), o fluxo de autenticação híbrida e o sistema de logs.
- **[Guia de Deploy](./docs/DEPLOYMENT.md)**: Como rodar com Docker, Docker Compose e CI/CD.
- **[Banco de Dados](./docs/DATABASE.md)**: Dependências de tabelas (`AD_***`), Views e Procedures necessárias no Oracle.
- **[Referência da API](./docs/API_Reference_and_Examples.md)**: Lista de endpoints (`/apiv1`), payloads JSON e exemplos de uso.

## 🚀 Quick Start (Local)

1. **Requisitos**: Go 1.23+, Docker e Redis.

2. **Configuração**:
   ```bash
   cp .env.example .env
   # Edite as variáveis SANKHYA_*, JWT_SECRET e REDIS_ADDR
   ```
3. **Rodar (Modo Dev com Hot-Reload)**:
   ```bash
   # Utilizando a configuração do docker-compose.yml fornecida
   docker-compose up -d
   ```


O servidor iniciará em `http://localhost:8080`.

## 🛠️ Stack Tecnológico

* **Core**: Go 1.25 (net/http, context, goroutines)
* **Database (Cache/Session)**: Redis (go-redis)
* **ERP Integration**: Sankhya (Service Layer + Direct SQL)
* **Logging**: Slog (JSON estruturado + Tint para console)

## 📄 Licença

Proprietário e Confidencial. Copyright © 2026 Roberto Casali Junior.
