FROM node:24.11 as builder

WORKDIR /build

COPY web/package.json .
COPY web/yarn.lock .

RUN yarn --frozen-lockfile

COPY ./web .
COPY ./VERSION .
RUN DISABLE_ESLINT_PLUGIN='true' VITE_APP_VERSION=$(cat VERSION) npm run build

FROM golang:1.25-alpine AS builder2
RUN sed -i "s@https://dl-cdn.alpinelinux.org/@https://mirrors.aliyun.com/@g" /etc/apk/repositories \
    && apk add --no-cache g++

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build
ADD go.mod go.sum ./
COPY . .
COPY --from=builder /build/build ./web/build
RUN go build -mod=vendor -ldflags "-s -w -X 'one-api/common.Version=$(cat VERSION)' -extldflags '-static'" -a -o one-api
RUN cd stress && go build -mod=vendor -a -o openai-mock

FROM docker.io/alpine:latest

RUN apk update \
    && apk upgrade \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates 2>/dev/null || true

COPY --from=builder2 /build/one-api /
COPY --from=builder2 /build/stress/openai-mock /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/one-api"]
