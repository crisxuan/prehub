FROM mirror.gcr.io/library/golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM scratch

WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker

EXPOSE 8080
CMD ["/app/api"]
