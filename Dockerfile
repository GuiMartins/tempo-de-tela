FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/tempo-de-tela ./cmd/tempo-de-tela

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/tempo-de-tela /tempo-de-tela
ENV TZ=America/Sao_Paulo \
    TDT_DATA=/appdata \
    TDT_DNS=:53 \
    TDT_HTTP=:8994 \
    TDT_LAN=1
VOLUME ["/appdata"]
EXPOSE 53/udp 53/tcp 8994/tcp
HEALTHCHECK --interval=60s --timeout=5s --start-period=10s CMD ["/tempo-de-tela", "-healthcheck"]
ENTRYPOINT ["/tempo-de-tela"]
