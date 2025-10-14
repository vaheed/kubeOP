# syntax=docker/dockerfile:1

FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ENV CGO_ENABLED=0
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=${VERSION} -X kubeop/internal/version.Commit=${COMMIT} -X kubeop/internal/version.Date=${DATE}" -o /out/kubeop-api ./cmd/api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=${VERSION} -X kubeop/internal/version.Commit=${COMMIT} -X kubeop/internal/version.Date=${DATE}" -o /out/kubeop-watcher ./cmd/kubeop-watcher

FROM gcr.io/distroless/base-debian12 AS watcher
COPY --from=build /out/kubeop-watcher /kubeop-watcher
EXPOSE 8081
ENTRYPOINT ["/kubeop-watcher"]

FROM gcr.io/distroless/static:nonroot AS api
WORKDIR /app
COPY --from=build /out/kubeop-api /app/api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]

