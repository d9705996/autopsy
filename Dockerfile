# ─── Stage 1: Build React SPA ─────────────────────────────────────────────────
FROM node:22-alpine AS ui-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci --include=dev
COPY web/ ./
RUN npm run build

# ─── Stage 2: Build Go binary ─────────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source (including ui/dist built in stage 1)
COPY . .
COPY --from=ui-builder /app/ui/dist ./ui/dist

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w \
        -X github.com/d9705996/autopsy/internal/version.Version=${VERSION} \
        -X github.com/d9705996/autopsy/internal/version.Commit=${COMMIT} \
        -X github.com/d9705996/autopsy/internal/version.Date=${DATE}" \
      -o /autopsy ./cmd/autopsy

# ─── Stage 3: Minimal runtime ─────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /autopsy /autopsy

EXPOSE 8080
ENTRYPOINT ["/autopsy"]
