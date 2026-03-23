# Example Dockerfile for a Go application (this is just a placeholder)
FROM scratch
COPY bsv-cmd-line-utils /
ENTRYPOINT ["/bsv-cmd-line-utils"]
