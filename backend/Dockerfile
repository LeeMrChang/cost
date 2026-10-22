ARG GO_IMAGE=golang:1.27-bookworm
ARG RUNTIME_IMAGE=debian:bookworm-slim
FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY="$GOPROXY" go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/cost-api ./cmd/api

FROM ${RUNTIME_IMAGE}
RUN apt-get update \
 && apt-get install --no-install-recommends -y ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/cost-api /usr/local/bin/cost-api
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/cost-api"]
