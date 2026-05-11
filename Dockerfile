FROM --platform=$BUILDPLATFORM golang:1.25.7 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /exporter .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /exporter /exporter

EXPOSE 48400

ENTRYPOINT ["/exporter"]
