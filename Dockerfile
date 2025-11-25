FROM golang:1.21 AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY controllers ./controllers
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /workspace/bin/manager ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
