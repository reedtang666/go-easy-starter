ARG GO_VERSION=1.26.10
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/starter .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 starter && adduser -S -D -u 10001 -G starter starter && \
    mkdir -p /app/storage/uploads /app/manifest/config && chown -R starter:starter /app
WORKDIR /app
COPY --from=build /out/starter /app/starter
COPY manifest/sql /app/manifest/sql
COPY manifest/config/config.example.yaml /app/manifest/config/config.example.yaml
USER 10001:10001
EXPOSE 8000
ENTRYPOINT ["/app/starter"]
CMD ["http"]
