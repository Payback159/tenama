FROM gcr.io/distroless/static@sha256:4b2a093ef4649bccd586625090a3c668b254cfe180dee54f4c94f3e9bd7e381e

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
