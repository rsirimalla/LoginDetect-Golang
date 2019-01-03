ARG GO_VERSION=1.11

# First stage: build the executable.
FROM golang:${GO_VERSION}-alpine AS builder
ENV CGO_ENABLED=0 GOFLAGS=-mod=vendor
WORKDIR /src

COPY ./ ./

# Build the executable to `/app`. Mark the build as statically linked.
RUN go build \
    -installsuffix 'static' \
    -o /app .

COPY ./GeoLite2-City.mmdb /app
COPY ./detector.db /app

# Final stage: the running container.
FROM scratch AS final

# Import the compiled executable from the second stage.
COPY --from=builder /app /app

EXPOSE 5000

# Run the compiled binary.
ENTRYPOINT ["/app"]