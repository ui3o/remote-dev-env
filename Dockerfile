FROM registry.fedoraproject.org/fedora:latest

# ARGS
ARG ARCH=amd64

# default configs
ENV CODE_SERVER_VERSION="4.12.0"

EXPOSE 8080

# add sudo privileges to podman
RUN echo "@includedir /etc/sudoers.d" >> /etc/sudoers
RUN echo "podman ALL=(ALL:ALL) NOPASSWD:ALL" >> /etc/sudoers.d/podman

# install base packages
RUN dnf -y update;\
    dnf -y reinstall shadow-utils;\
    dnf -y install man openssh openssh-clients ca-certificates gnupg net-tools git-lfs cmatrix cowsay htop sssd sssd-tools procps-ng ncdu xz ranger wget zsh git neovim tmux fzf make tree unzip systemd podman fuse-overlayfs --exclude container-selinux;\
    rm -rf /var/cache /var/log/dnf* /var/log/yum.*

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

