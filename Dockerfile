FROM docker.io/library/golang:1 AS builder

WORKDIR /app

# go mod first to allow dependency caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1

RUN go build -o rae-wod .

FROM gcr.io/distroless/base:nonroot

COPY --from=builder /app/rae-wod /app/rae-wod

ENTRYPOINT [ "/app/rae-wod" ]
