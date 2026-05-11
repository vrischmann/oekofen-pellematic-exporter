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
    podman build \
        --platform linux/amd64,linux/arm64 \
        --format docker \
        -t {{ image }}:{{ tag }} \
        --push \
        .

# Lint the code
lint:
    staticcheck ./...
