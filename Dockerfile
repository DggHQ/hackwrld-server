FROM golang:alpine as builder-base
LABEL builder=true multistage_tag="gamemaster"
RUN apk add --no-cache upx ca-certificates tzdata

FROM builder-base as builder-modules
LABEL builder=true multistage_tag="gamemaster"
ARG TARGETARCH
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
RUN go mod verify

FROM builder-modules as builder
LABEL builder=true multistage_tag="gamemaster"
ARG TARGETARCH
WORKDIR /build
COPY *.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags '-s -w -extldflags="-static"' -v -o gamemaster
RUN upx --best --lzma gamemaster 

FROM alpine:3.17
WORKDIR /app
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/gamemaster /usr/bin/
CMD ["gamemaster"]