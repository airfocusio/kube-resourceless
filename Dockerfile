FROM scratch
ENTRYPOINT ["/bin/kube-resourceless"]
COPY kube-resourceless /bin/kube-resourceless
WORKDIR /workdir