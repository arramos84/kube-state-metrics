FROM gcr.io/distroless/static

COPY kube-event-exporter /

USER nobody

ENTRYPOINT ["/kube-event-exporter", "--port=8080", "--telemetry-port=8081"]

EXPOSE 8080 8081
