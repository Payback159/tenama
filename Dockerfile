# build stage
# golang:1.20.3-bullseye
FROM golang@sha256:6a09d7e431f3a2e263c6e7f14f26db634f2f707b8f3efb7255a54d9ff2c6ee3a AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/handlers.commit=$(git rev-parse --verify HEAD)'" \
    .

# final stage
FROM gcr.io/distroless/static@sha256:a01d47d4036cae5a67a9619e3d06fa14a6811a2247b4da72b4233ece4efebd57
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /.docs

CMD ["/tenama"]
