# ---------- build stage ----------
FROM --platform=linux/amd64 golang:1.23-bookworm AS builder

WORKDIR /app

# CGO 编译工具
RUN apt-get update && apt-get install -y \
    build-essential \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 依赖缓存
COPY go.mod go.sum ./
RUN go mod download

# 代码
COPY . .

# 编译（与你本地一致：CGO=1）
ENV CGO_ENABLED=1
RUN go build -o server main.go


# ---------- run stage ----------
FROM --platform=linux/amd64 debian:bookworm-slim
WORKDIR /app

# 运行时证书
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server /app/server
COPY config /app/config
COPY .env /app/.env

EXPOSE 18080

CMD ["/app/server"]