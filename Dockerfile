# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS web-builder
WORKDIR /workspace

COPY package.json package-lock.json ./
RUN npm ci

COPY index.html tsconfig.json vite.config.ts ./
COPY src ./src
RUN npm run build

FROM golang:1.26.5-alpine AS go-builder
WORKDIR /workspace/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/. ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21 AS runtime
WORKDIR /app

RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates

ARG APP_VERSION=v0.4.2
ARG APP_COMMIT=local
ARG APP_BUILT_AT=unknown

ENV PORT=3000
ENV DIST_DIR=dist
ENV APP_MODE=demo
ENV DEV_MODE=false
ENV AUTH_ENABLED=false
ENV WRITE_ACTIONS_ENABLED=false
ENV APP_VERSION=$APP_VERSION
ENV APP_COMMIT=$APP_COMMIT
ENV APP_BUILT_AT=$APP_BUILT_AT

COPY --from=go-builder /out/server /app/server
COPY --from=web-builder /workspace/dist /app/dist

EXPOSE 3000
USER app

ENTRYPOINT ["/app/server"]
