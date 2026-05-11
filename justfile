image := "registry.tail04bc6.ts.net/oekofen-pellematic-exporter"
tag := "latest"

# Build and push the multi-arch Docker image
build:
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        -t {{ image }}:{{ tag }} \
        --push \
        .

# Build and push the multi-arch Podman image
build-podman:
    podman pull golang:1.25.7
    podman pull gcr.io/distroless/static-debian12:nonroot

    # Build the manifest
    podman build \
        --platform linux/amd64,linux/arm64 \
        --manifest {{ image }}:{{ tag }} \
        .

    # Push the manifest to the registry
    podman manifest push {{ image }}:{{ tag }}

    # Clean up the local manifest
    podman manifest rm {{ image }}:{{ tag }} || true

# Lint the code
lint:
    staticcheck ./...
