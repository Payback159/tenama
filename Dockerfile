# build stage
ARG GO_VERS=1.19.0
FROM golang:${GO_VERS}-alpine AS build-env

ADD certs/ /usr/local/share/ca-certificates/
RUN update-ca-certificates

RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o tenama .

# final stage
FROM gcr.io/distroless/static
COPY --from=build-env /build/tenama /
COPY --from=build-env /build/.docs /docs

CMD ["/tenama"]
