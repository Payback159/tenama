FROM gcr.io/distroless/static@sha256:d90359c7a3ad67b3c11ca44fd5f3f5208cbef546f2e692b0dc3410a869de46bf

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
