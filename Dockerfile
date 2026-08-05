# syntax=docker/dockerfile:1
FROM node:22-alpine AS web-build
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/paperwing ./cmd/paperwing && \
    mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/paperwing /paperwing
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/paperwing"]
