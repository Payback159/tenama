# build stage
# golang:1.21.4-bookworm
FROM golang@sha256:34ce21a9696a017249614876638ea37ceca13cdd88f582caad06f87a8aa45bf3 AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama \
    -ldflags="-X 'github.com/Payback159/tenama/internal/handlers.version=$(git describe --tags)' -X 'github.com/Payback159/tenama/internal/handlers.builddate=$(date)' -X 'github.com/Payback159/tenama/internal/handlers.commit=$(git rev-parse --verify HEAD)'" \
    ./cmd/tenama

# final stage

FROM gcr.io/distroless/static@sha256:a43abc840a7168c833a8b3e4eae0f715f7532111c9227ba17f49586a63a73848
ENV BUILDDIR=/build
COPY --from=build-env $BUILDDIR/tenama /
COPY --from=build-env $BUILDDIR/api /api
COPY --from=build-env $BUILDDIR/web /web

CMD ["/tenama"]
