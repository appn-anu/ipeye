FROM alpine:latest
RUN apk add --no-cache tzdata
COPY ipeye /bin
VOLUME /data
ENTRYPOINT ["ipeye"]
