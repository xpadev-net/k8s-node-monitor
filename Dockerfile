# ビルドステージ
FROM golang:1.20-alpine AS builder

WORKDIR /app

# 依存関係のインストール
COPY go.mod go.sum ./
RUN go mod download

# ソースコードのコピーとビルド
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k8s-node-monitor .

# 実行ステージ
FROM alpine:3.18

# 証明書のインストール（HTTPS通信に必要）
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/k8s-node-monitor .

# 実行
CMD ["./k8s-node-monitor"]
