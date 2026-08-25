FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wasm-sandbox ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/wasm-sandbox /app/wasm-sandbox
COPY configs/config.yaml /app/configs/config.yaml
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/app/wasm-sandbox"]
CMD ["-config", "/app/configs/config.yaml"]
