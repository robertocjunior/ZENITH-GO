# Estágio de Compilação (Builder)
# Usamos uma imagem Go leve baseada em Alpine
FROM golang:1.23-alpine AS builder

# Instala git (necessário para baixar dependências)
RUN apk add --no-cache git

# Define o diretório de trabalho dentro do container
WORKDIR /app

# Copia os arquivos de dependência primeiro (para cachear camadas do Docker)
COPY go.mod go.sum ./

# Baixa as dependências
RUN go mod download

# Copia o restante do código fonte
COPY . .

# Compila a aplicação
# CGO_ENABLED=0 garante um binário estático puro
# GOOS=linux define o sistema operacional alvo
RUN CGO_ENABLED=0 GOOS=linux go build -o zenith-api ./cmd/api/main.go

# ---------------------------------------------------------

# Estágio Final (Runner)
# Usamos uma imagem Alpine limpa e minúscula para rodar
FROM alpine:latest

# Instala certificados CA (para HTTPS) e dados de Timezone (para logs corretos)
RUN apk --no-cache add ca-certificates tzdata

# Define o fuso horário (Opcional, exemplo: São Paulo)
ENV TZ=America/Sao_Paulo

WORKDIR /root/

# Copia o binário compilado do estágio anterior
COPY --from=builder /app/zenith-api .

# Cria a pasta de logs para garantir que o volume funcione
RUN mkdir logs

# Expõe a porta da aplicação
EXPOSE 8080

# Comando para iniciar a aplicação
CMD ["./zenith-api"]