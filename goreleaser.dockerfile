FROM gcr.io/distroless/static@sha256:69830f29ed7545c762777507426a412f97dad3d8d32bae3e74ad3fb6160917ea

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
