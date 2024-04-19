FROM golang:alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

ARG TARGETARCH
ENV GOARCH=$TARGETARCH

WORKDIR /opt

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src .

RUN go build -ldflags "-s -w" -o extensions/aws-lambda-loki-extension main.go
RUN chmod -R 755 extensions/aws-lambda-loki-extension

FROM scratch
WORKDIR /opt/extensions
COPY --from=builder /opt/extensions .
