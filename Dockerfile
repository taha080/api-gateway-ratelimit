# Multi-stage build: tiny static image, works on Render's free web service plan.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /gateway /gateway
COPY config.yaml /config.yaml
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/gateway", "-config", "/config.yaml"]
