FROM golang:alpine AS builder

ARG ARCH
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${ARCH:-amd64}

WORKDIR /build

COPY . .

RUN go mod download
RUN go build -o extensions/aws-lambda-loki-extension main.go
RUN chmod +x extensions/aws-lambda-loki-extension

FROM scratch
COPY --from=builder /build/extensions/aws-lambda-loki-extension /opt/extensions
