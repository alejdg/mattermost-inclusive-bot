FROM ubuntu:focal 

ENV GOPATH=/usr/local/go
ENV GOPATH=/root/go
ENV PATH=/usr/local/go/bin:$PATH:$GOPATH/bin

RUN --mount=type=cache,target=/var/cache/apt \
    apt-get -y update && \
    apt-get -y dist-upgrade && \
    apt-get -y --no-install-recommends install \
        wget \
        gcc \
        make \
        ca-certificates && \
    update-ca-certificates && \
    apt-get -y --no-install-recommends install \
        git

ENV GO_VERSION=1.20
RUN wget -P /tmp "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz"
RUN tar -C /usr/local -xzvf "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
RUN rm "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

COPY app/go.mod app/go.sum app/ 
WORKDIR /app
RUN go mod download

WORKDIR /app
COPY app .
RUN CGO_ENABLED=0 go build . 

WORKDIR /app
# ENTRYPOINT ["./mattermost-inclusive-bot"]
CMD ["./mattermost-inclusive-bot"]

# Websocket port
EXPOSE 8066