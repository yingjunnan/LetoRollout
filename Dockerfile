# --- web: build the React frontend ---
FROM node:20-slim AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
# Build output lands in ../internal/httpapi/static (see vite.config.ts).
RUN npm run build

# --- go: build the server (embeds the web build) ---
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Copy the whole repo, including the Vite build output from the web stage.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/letorollout ./cmd/server

FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=builder /out/letorollout /letorollout
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/letorollout"]
