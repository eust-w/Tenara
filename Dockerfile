# Multi-module build: pass MODULE=<go module dir> to build one binary.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.work ./
COPY control-plane/go.mod control-plane/go.sum* ./control-plane/
COPY controllers/go.mod controllers/go.sum* ./controllers/
COPY analyzer/go.mod analyzer/go.sum* ./analyzer/
COPY builder/go.mod builder/go.sum* ./builder/
COPY verifier/go.mod verifier/go.sum* ./verifier/
COPY providers/go.mod providers/go.sum* ./providers/
RUN go work sync || true
COPY control-plane ./control-plane
COPY controllers ./controllers
ARG MODULE=control-plane
RUN cd "$MODULE" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
