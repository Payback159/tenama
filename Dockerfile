# build stage
# golang:1.20.4-bullseye
FROM golang@sha256:6b3fa4b908676231b50acbbc00e84d8cee9c6ce072b1175c0ff352c57d8a612f AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/handlers.commit=$(git rev-parse --verify HEAD)'" \
    .

# final stage
FROM gcr.io/distroless/static@sha256:7198a357ff3a8ef750b041324873960cf2153c11cc50abb9d8d5f8bb089f6b4e
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /.docs

CMD ["/tenama"]
