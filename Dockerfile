FROM registry.fedoraproject.org/fedora:latest

# ARGS
ARG ARCH=amd64

# default configs
ENV CODE_SERVER_VERSION="4.12.0"

EXPOSE 8080

# add sudo privileges to podman
RUN echo "@includedir /etc/sudoers.d" >> /etc/sudoers
RUN echo "podman ALL=(ALL:ALL) NOPASSWD:ALL" >> /etc/sudoers.d/podman

COPY ./.config/user/ /home/podman/
COPY ./.config/etc/ /etc/
COPY ./.config/user/.gitconfig /root/.gitconfig
COPY ./.config/root/ /root/

# RUN install.sh
RUN /etc/install.sh
# change user to root
USER root

VOLUME /home/podman/.local/share/containers

RUN chown podman:podman -R /home/podman

ENV _CONTAINERS_USERNS_CONFIGURED=""

STOPSIGNAL SIGRTMIN+3

WORKDIR /home/podman
ENTRYPOINT ["/etc/systemd/system/container-entrypoint.sh"]

