FROM gcr.io/distroless/static@sha256:5c7e2b465ac6a2a4e5f4f7f722ce43b147dabe87cb21ac6c4007ae5178a1fa58

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
