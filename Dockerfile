FROM golang:1.20 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/letorollout ./cmd/server

FROM gcr.io/distroless/static-debian11:nonroot

COPY --from=builder /out/letorollout /letorollout

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/letorollout"]
