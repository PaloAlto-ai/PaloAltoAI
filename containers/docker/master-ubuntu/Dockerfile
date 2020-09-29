FROM ubuntu:xenial

ENV PATH=/usr/lib/go-1.9/bin:$PATH

RUN \
  apt-get update && apt-get upgrade -q -y && \
  apt-get install -y --no-install-recommends golang-1.9 git make gcc libc-dev ca-certificates && \
  git clone --depth 1 --branch release/1.8 https://github.com/PaloAltoAi/go-PaloAltoAi && \
  (cd go-PaloAltoAi && make gpaa) && \
  cp go-PaloAltoAi/build/bin/gpaa /gpaa && \
  apt-get remove -y golang-1.9 git make gcc libc-dev && apt autoremove -y && apt-get clean && \
  rm -rf /go-PaloAltoAi

EXPOSE 8545
EXPOSE 30303

ENTRYPOINT ["/gpaa"]
