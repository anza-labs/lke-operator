#########################################################################################
# Build
#########################################################################################

# First stage: building the driver executable.
FROM --platform=${TARGETPLATFORM} docker.io/library/golang:1.22.3 AS builder
ARG TARGETOS
ARG TARGETARCH

# Set the working directory.
WORKDIR /workspace

# Copy the Go Modules manifests.
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as
# much and so that source changes don't invalidate our downloaded layer.
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Arg for setting version.
ARG VERSION=canary

# Build.
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
RUN go build \
        -trimpath \
        -ldflags="-X github.com/anza-labs/lke-operator/internal/version.Version=${VERSION} -s -w -extldflags '-static'" \
        -o=./bin/manager \
        ./cmd/main.go

#########################################################################################
# Runtime
#########################################################################################

# Second stage: building final environment for running the executable.
# hadolint ignore=DL3007
FROM gcr.io/distroless/static:latest AS runtime

# Copy the executable.
COPY --from=builder --chown=65532:65532 /workspace/bin/manager /manager

# Set the final UID:GID to non-root user.
USER 65532:65532

# Disable healthcheck.
HEALTHCHECK NONE

# Add labels.
ARG VERSION=canary

## Standard opencontainers labels.
LABEL org.opencontainers.image.title="lke-operator"
LABEL org.opencontainers.image.description="Kubernetes operator for managing LKE (Linode Kubernetes Engine) instances."
LABEL org.opencontainers.image.authors="anza-labs contributors"
LABEL org.opencontainers.image.vendor="anza-labs"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.license="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/anza-labs/lke-operator"
LABEL org.opencontainers.image.documentation="http://anza-labs.github.io/lke-operator"
LABEL org.opencontainers.image.base.name="gcr.io/distroless/static:latest"

# Set the entrypoint.
ENTRYPOINT [ "/manager" ]
CMD []
