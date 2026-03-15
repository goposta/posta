# UI build stage
FROM node:22-alpine AS ui-builder

WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Go build stage
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=2026-03-15

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X 'github.com/jkaninda/posta/internal/config.Version=${VERSION}' -X 'github.com/jkaninda/posta/internal/config.CommitID=${COMMIT}' -X 'github.com/jkaninda/posta/internal/config.BuildDate=${BUILD_DATE}'" \
    -o /bin/posta ./cmd/posta

# Runtime stage
FROM alpine:3.21

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="posta" \
      org.opencontainers.image.description="Posta is a self-hosted email delivery platform" \
      org.opencontainers.image.licenses="Apache" \
      org.opencontainers.image.authors="Jonas Kaninda" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/jkaninda/posta" \
      org.opencontainers.image.revision="${COMMIT}"

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S posta && adduser -S posta -G posta

COPY --from=builder /bin/posta /bin/posta
RUN ln -s /bin/posta /posta
COPY --from=ui-builder /app/web/dist /app/web/dist

ENV POSTA_WEB_DIR=/app/web/dist


USER posta
EXPOSE 9000

ENTRYPOINT ["/bin/posta"]
