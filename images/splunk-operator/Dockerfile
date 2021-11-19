
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
WORKDIR /
ARG BIN=manager
COPY $BIN /workspace/manager 

ENTRYPOINT ["/manager"]