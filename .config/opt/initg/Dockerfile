FROM alpine

RUN apk add htop

COPY main /main

ENTRYPOINT [ "/main", "sleep", "999" ]