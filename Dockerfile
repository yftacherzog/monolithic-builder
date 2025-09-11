# Multi-stage build for single monolithic builder binary
FROM registry.access.redhat.com/ubi9/go-toolset:9.6-1752083840 AS builder
ARG TARGETOS
ARG TARGETARCH

ENV GOTOOLCHAIN=auto
WORKDIR /workspace

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build single unified binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -buildvcs=false -o /opt/app-root/monolithic-builder ./cmd/monolithic-builder

# Runtime image - use UBI 10 like official Konflux buildah images
FROM registry.access.redhat.com/ubi10/ubi

# Install all required packages for both tasks
RUN dnf update -y && \
    dnf install -y \
        buildah \
        skopeo \
        ca-certificates && \
    dnf clean all

# Copy unified binary
COPY --from=builder /opt/app-root/monolithic-builder /usr/local/bin/monolithic-builder

# Create symlinks for backward compatibility
RUN ln -s /usr/local/bin/monolithic-builder /usr/local/bin/build-container && \
    ln -s /usr/local/bin/monolithic-builder /usr/local/bin/build-image-index

# Set up buildah storage and configuration
RUN mkdir -p /var/lib/containers /workspace /tekton/results /.docker && \
    chmod 755 /var/lib/containers /.docker && \
    chown root:root /var/lib/containers

# Configure registries - disable short-name-mode for security
RUN sed -i 's/^\s*short-name-mode\s*=\s*.*/short-name-mode = "disabled"/' /etc/containers/registries.conf

# Set up user namespace for rootless buildah - 2^32-2
RUN echo 'root:1:4294967294' | tee -a /etc/subuid >> /etc/subgid

# Set up environment for both use cases
ENV BUILDAH_ISOLATION=chroot
ENV STORAGE_DRIVER=vfs

# Default entrypoint - can be overridden by task definition
ENTRYPOINT ["/usr/local/bin/monolithic-builder", "build-container"]
