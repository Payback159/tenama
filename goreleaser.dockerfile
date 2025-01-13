FROM gcr.io/distroless/static@sha256:3f2b64ef97bd285e36132c684e6b2ae8f2723293d09aae046196cca64251acac

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
