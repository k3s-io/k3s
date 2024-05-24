FROM ubuntu:24.04
ARG EXTERNAL_ENCODED_VPN
ARG VPN_ENCODED_LOGIN

RUN apt-get update && \
    apt-get install -y curl bridge-utils iputils-ping openvpn openssh-client && \
    mkdir -p /dev/net && \
    mknod /dev/net/tun c 10 200 && \
    chmod 600 /dev/net/tun

RUN if [[ -z "$EXTERNAL_ENCODED_VPN" ]] ; then echo "no vpn provided" ;  \
    else echo -n $EXTERNAL_ENCODED_VPN | base64 -di > external.ovpn && \
    if [[ -z "$VPN_ENCODED_LOGIN" ]]; then echo "no passcode provided" ; \
    else echo -n $VPN_ENCODED_LOGIN | base64 -di > authfile && \
    sed -i 's/auth-user-pass/auth-user-pass authfile/g' external.ovpn; fi ; fi

WORKDIR .
COPY scripts/run_tests.sh .
COPY scripts/init.sh .
