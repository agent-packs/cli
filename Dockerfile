FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY cli/ ./cli/
COPY skills/ ./skills/
COPY registry/ ./registry/
WORKDIR /src/cli
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
      -X github.com/sandeshh/agent-packs/cli/internal/version.Version=${VERSION} \
      -X github.com/sandeshh/agent-packs/cli/internal/version.Commit=${COMMIT}" \
    -o /agent-packs ./cmd/agent-packs

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=builder /agent-packs /usr/local/bin/agent-packs
COPY --from=builder /src/registry /registry
COPY --from=builder /src/skills /skills
# Pack manifests live under registry/packs; point the CLI at them.
ENV AGENT_PACKS_REGISTRY=/registry/packs
ENTRYPOINT ["agent-packs"]
CMD ["--help"]
