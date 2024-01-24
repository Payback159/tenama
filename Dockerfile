# build stage
# golang:1.21.4-bookworm
FROM golang@sha256:76aadd914a29a2ee7a6b0f3389bb2fdb87727291d688e1d972abe6c0fa6f2ee0 AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/internal/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/internal/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/internal/handlers.commit=$(git rev-parse --verify HEAD)'" \
    ./cmd/tenama

# final stage

FROM gcr.io/distroless/static@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25
ENV BUILDDIR=/build
COPY --from=build-env $BUILDDIR/tenama /
COPY --from=build-env $BUILDDIR/api /api
COPY --from=build-env $BUILDDIR/web /web

CMD ["/tenama"]
