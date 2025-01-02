FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-gitlab"]
COPY baton-gitlab /