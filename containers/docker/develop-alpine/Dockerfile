FROM alpine:3.7

RUN \
  apk add --update go git make gcc musl-dev linux-headers ca-certificates && \
  git clone --depth 1 https://github.com/PaloAltoAi/go-PaloAltoAi && \
  (cd go-PaloAltoAi && make gpaa) && \
  cp go-PaloAltoAi/build/bin/gpaa /gpaa && \
  apk del go git make gcc musl-dev linux-headers && \
  rm -rf /go-PaloAltoAi && rm -rf /var/cache/apk/*

EXPOSE 8545
EXPOSE 30303

ENTRYPOINT ["/gpaa"]
