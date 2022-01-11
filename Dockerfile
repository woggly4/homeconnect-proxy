##
## Build
##
FROM golang:1.17.6-stretch AS builder

# Copy app files
WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./

# Build app
RUN go build -o /app

##
## Deploy
##
FROM gcr.io/distroless/static AS final
USER nonroot:nonroot

# Copy built binary from builder
COPY --from=builder --chown=nonroot:nonroot /app /app

# Expose port
EXPOSE 8088
# Exec built binary
CMD ./homeconnect-proxy

ENTRYPOINT ["/homeconnect-proxy"]