FROM debian:latest

RUN apt update && apt install -y dnsdist curl dnsutils tcpdump && \
    mkdir -p /var/log/dnsdist

COPY dnsdist.conf /etc/dnsdist/dnsdist.conf

EXPOSE 53/udp 53/tcp 8083/tcp 5199/tcp

CMD [ "dnsdist", "-C", "/etc/dnsdist/dnsdist.conf", "--supervised" ]
