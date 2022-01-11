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
# CGO_ENABLED: https://stackoverflow.com/questions/36279253/go-compiled-binary-wont-run-in-an-alpine-docker-container-on-ubuntu-host
RUN CGO_ENABLED=0 go build -o /app

# Create empty files we need so we can then copy into the distroless container
RUN touch /app.log
RUN touch /token.cache

##
## Deploy
##
FROM gcr.io/distroless/static AS final

# Change user
USER nonroot:nonroot

# Copy the app files for the builder and set access
COPY --from=builder --chown=nonroot:nonroot /app.log /
COPY --from=builder --chown=nonroot:nonroot /token.cache /

# The application logfile app.log can be exposed on host via single file mapping
# It must be created before running the container and owned by container's nonroot user (uid 65532)
# sudo chown 65532 app.log

# Copy built binary from builder
COPY --from=builder --chown=nonroot:nonroot /app /      

# Expose port
EXPOSE 8088

# Exec built binary
CMD ["/app"]