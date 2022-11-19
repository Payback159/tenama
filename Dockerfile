# build stage
FROM golang@sha256:0eb08c89ab1b0c638a9fe2780f7ae3ab18f6ecda2c76b908e09eb8073912045d AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama .

# final stage
FROM gcr.io/distroless/static@sha256:5759d194607e472ff80fff5833442d3991dd89b219c96552837a2c8f74058617
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /.docs

CMD ["/tenama"]
