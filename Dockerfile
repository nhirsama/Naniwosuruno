# Build stage
FROM golang:1.24-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG ALL_PROXY

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/cli/ ./cli/
COPY pkg/ ./pkg/
COPY internal/server/ ./server/
COPY internal/client/ ./client/
COPY main.go ./

ENV CGO_ENABLED=0
RUN go build -o naniwosuruno cmd/naniwosuruno/main.go


FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/naniwosuruno .

COPY index.html .

RUN mkdir -p data

EXPOSE 9975

VOLUME ["/app/data"]

CMD ["./naniwosuruno", "server"]
