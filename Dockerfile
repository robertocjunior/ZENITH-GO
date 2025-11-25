# Estágio de Compilação (Builder)
FROM golang:1.23-alpine AS builder

# ATUALIZAÇÃO DE SEGURANÇA: Atualiza pacotes do sistema base
RUN apk upgrade --no-cache

# Instala git
RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o zenith-api ./cmd/api/main.go

# ---------------------------------------------------------

# Estágio Final (Runner)
FROM alpine:latest

# ATUALIZAÇÃO DE SEGURANÇA
RUN apk upgrade --no-cache

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=America/Sao_Paulo

WORKDIR /root/

COPY --from=builder /app/zenith-api .

RUN mkdir logs

EXPOSE 8080

CMD ["./zenith-api"]