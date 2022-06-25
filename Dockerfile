FROM scratch
EXPOSE 8080
COPY hdg-exporter /hdg-exporter
COPY app.env /app.env
ENTRYPOINT ["/hdg-exporter"]
