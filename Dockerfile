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
RUN mkdir /data
RUN touch /data/app.log
RUN touch /data/token.cache


##
## Deploy
##
FROM gcr.io/distroless/static AS final

# Change user
#USER nonroot:nonroot

# The application logfile app.log can be exposed on host via single file mapping
# It must be created before running the container and owned by container's nonroot user (uid 65532)
#RUN sudo chown 65532 /data
#RUN sudo chown 65532 /data/app.log
#RUN sudo chown 65532 /data/token.cache

# Copy built binary from builder
COPY --from=builder /app /

# Copy the app files for the builder and set access
COPY --from=builder --chown=root:root /data /data

# Expose port
EXPOSE 8088

# Exec built binary
CMD ["/app"]
