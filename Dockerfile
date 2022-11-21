FROM golang:1.19 AS builder
WORKDIR /hdg-exporter
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o hdg-exporter

FROM scratch
EXPOSE 8080
COPY --from=builder /hdg-exporter/hdg-exporter /hdg-exporter
COPY --from=builder /hdg-exporter/app.env /app.env

ENTRYPOINT ["/hdg-exporter"]
