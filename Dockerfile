# build stage
# golang:1.20.1-bullseye
FROM golang@sha256:c3fbdc381fb6b78325c2a5cc1bf0c288c0d173568fba3f1b8894a51837cccf7f AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/handlers.commit=$(git rev-parse --verify HEAD)'" \
    .

# final stage
FROM gcr.io/distroless/static@sha256:5759d194607e472ff80fff5833442d3991dd89b219c96552837a2c8f74058617
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /.docs

CMD ["/tenama"]
