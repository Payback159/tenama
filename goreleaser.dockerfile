FROM gcr.io/distroless/static@sha256:47b2d72ff90843eb8a768b5c2f89b40741843b639d065b9b937b07cd59b479c6

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
