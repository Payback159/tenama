# build stage
# golang:1.19.4-bullseye
FROM golang@sha256:04f76f956e51797a44847e066bde1341c01e09054d3878ae88c7f77f09897c4d AS build-env

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
