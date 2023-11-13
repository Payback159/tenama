# build stage
# golang:1.21.1-bookworm
FROM golang@sha256:81cd210ae58a6529d832af2892db822b30d84f817a671b8e1c15cff0b271a3db AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/handlers.commit=$(git rev-parse --verify HEAD)'" \
    .

# final stage
FROM gcr.io/distroless/static@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /.docs

CMD ["/tenama"]
