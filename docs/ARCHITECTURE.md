# Arquitetura do Sistema

## Estrutura do Projeto
O projeto segue o padrão **Standard Go Project Layout**:

| Diretório | Descrição |
|-----------|-----------|
| `cmd/api` | Ponto de entrada (`main.go`). Inicializa config, conexões e servidor HTTP. |
| `internal/auth` | Gerenciamento de JWT e Sessão Redis. Implementa a lógica de *Sliding Expiration*. |
| `internal/sankhya` | Cliente HTTP para o ERP. Contém a lógica de *Retry*, *Keep-Alive* e queries SQL. |
| `internal/handler` | Camada HTTP. Recebe requests, valida JSON e chama os serviços internos. |
| `internal/logger` | Sistema de logs customizado. |
| `internal/notification` | Serviço de e-mail (SMTP) para alertas críticos (Panics/Errors 500). |

## 🔐 Fluxo de Autenticação Híbrida
O sistema utiliza duas camadas de segurança simultâneas:

1. **Mobile (Usuário Final)**:
   - O usuário loga com usuário/senha do Sankhya.
   - O sistema valida no ERP e gera um **JWT** interno.
   - Este JWT é vinculado a uma sessão no **Redis**.
   - **Controle de Dispositivo**: Verifica a tabela `AD_DISPAUT` para impedir login em coletores não autorizados.

2. **System (Service Account)**:
   - O backend mantém uma sessão "invisível" com o Sankhya usando um usuário de integração (definido no `.env`).
   - O `client.go` gerencia a renovação automática do Token Bearer do sistema caso expire.

## 📡 Integração com Sankhya
A comunicação é feita via **Sankhya Service Layer (MGE)**:
- **Leitura**: Usa o serviço `DbExplorerSP.executeQuery` para rodar SQL direto no banco Oracle.
- **Escrita**: Usa `DatasetSP.save` para manipulação de tabelas customizadas (`AD_BXAEND`, `AD_IBXEND`).
- **Procedures**: Dispara ações de negócio via `ActionButtonsSP.executeSTP`.

## 📝 Sistema de Logs (Hybrid Logger)
Implementado em `internal/logger/logger.go`, utiliza um **Fanout Handler**:
1. **Console**: Saída colorida e human-readable (para desenvolvimento).
2. **Arquivo (`logs/zenith.log`)**: Formato JSON estruturado, com rotação automática baseada em tamanho (100MB) ou data, ideal para ingestão em ferramentas como ELK/Datadog.
