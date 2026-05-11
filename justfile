image := "registry.tail04bc6.ts.net/oekofen-pellematic-exporter"
tag := "latest"

# Build and push the multi-arch Docker image
build:
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        -t {{ image }}:{{ tag }} \
        --push \
        .

# Lint the code
lint:
    staticcheck ./...
