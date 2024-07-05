FROM gcr.io/distroless/static@sha256:ce46866b3a5170db3b49364900fb3168dc0833dfb46c26da5c77f22abb01d8c3

COPY tenama /
COPY api/ /api/
COPY web/ /web/

CMD ["/tenama"]
