FROM gcr.io/distroless/static@sha256:b033683de7de51d8cce5aa4b47c1b9906786f6256017ca8b17b2551947fcf6d8

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
