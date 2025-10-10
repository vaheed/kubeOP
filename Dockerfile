# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod ./
COPY go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ENV CGO_ENABLED=0
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=${VERSION} -X kubeop/internal/version.Commit=${COMMIT} -X kubeop/internal/version.Date=${DATE}" -o /out/kubeop cmd/api/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/kubeop /app/api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]

