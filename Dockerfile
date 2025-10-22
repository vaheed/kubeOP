# syntax=docker/dockerfile:1

FROM golang:1.24.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY kubeop-operator/go.mod kubeop-operator/go.sum ./kubeop-operator/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ENV CGO_ENABLED=0
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w -X kubeop/internal/version.rawVersion=${VERSION} -X kubeop/internal/version.rawCommit=${COMMIT} -X kubeop/internal/version.rawDate=${DATE}" -o /out/kubeop-api ./cmd/api

FROM golang:1.24.3 AS operator-build
WORKDIR /src/kubeop-operator
COPY kubeop-operator/go.mod kubeop-operator/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY kubeop-operator/ ./
ENV CGO_ENABLED=0
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /out/kubeop-operator-manager ./cmd/manager

FROM gcr.io/distroless/static:nonroot AS api
WORKDIR /app
COPY --from=build /out/kubeop-api /app/api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]

FROM gcr.io/distroless/static:nonroot AS operator
WORKDIR /app
COPY --from=operator-build /out/kubeop-operator-manager /app/manager
EXPOSE 8080 8081
USER nonroot:nonroot
ENTRYPOINT ["/app/manager"]

