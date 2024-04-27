# build stage
# golang:1.21.4-bookworm
FROM golang@sha256:450e3822c7a135e1463cd83e51c8e2eb03b86a02113c89424e6f0f8344bb4168 AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/internal/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/internal/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/internal/handlers.commit=$(git rev-parse --verify HEAD)'" \
    ./cmd/tenama

# final stage

FROM gcr.io/distroless/static@sha256:6d31326376a7834b106f281b04f67b5d015c31732f594930f2ea81365f99d60c
ENV BUILDDIR=/build
COPY --from=build-env $BUILDDIR/tenama /
COPY --from=build-env $BUILDDIR/api /api
COPY --from=build-env $BUILDDIR/web /web

CMD ["/tenama"]
