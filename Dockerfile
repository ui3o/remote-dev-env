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
    dnf -y install rsync man openssh openssh-clients ca-certificates gnupg net-tools git-lfs cmatrix cowsay htop sssd sssd-tools procps-ng ncdu xz ranger wget zsh git neovim tmux fzf make tree unzip systemd podman fuse-overlayfs --exclude container-selinux;\
    rm -rf /var/cache /var/log/dnf* /var/log/yum.*

COPY ./.config/user/ /home/podman/
COPY ./.config/etc/ /etc/
COPY ./.config/user/.gitconfig /root/.gitconfig
COPY ./.config/root/ /root/

# RUN install.sh
# RUN /etc/install.sh
# setup podman user
RUN echo start install on $ARCH architecture... && \
    echo [$ARCH] setup podman user... && \
    useradd podman && \
    echo podman:10000:5000 >/etc/subuid && \
    echo podman:10000:5000 >/etc/subgid && \
    usermod --shell /usr/bin/zsh podman

# setup file system for podman
RUN echo [$ARCH] setup file system for podman... && \
    mkdir -p /home/podman/.config/containers && \
    curl -fL https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/containers.conf -o /etc/containers/containers.conf && \
    curl -fL https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/podman-containers.conf -o /home/podman/.config/containers/containers.conf && \
    chmod 644 /etc/containers/containers.conf && \
    mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers && \
    touch /var/lib/shared/overlay-images/images.lock && \
    touch /var/lib/shared/overlay-layers/layers.lock && \
    touch /var/lib/shared/vfs-images/images.lock && \
    touch /var/lib/shared/vfs-layers/layers.lock

RUN echo [$ARCH] setup vscode-server... && \
    curl -fL https://github.com/coder/code-server/releases/download/v$CODE_SERVER_VERSION/code-server-$CODE_SERVER_VERSION-$ARCH.rpm -o /tmp/code-server.rpm && \
    rpm -i /tmp/code-server.rpm && \
    cd /usr/lib/code-server/src/browser/pages && \
    curl -O "https://demyx.sh/fonts/{meslolgs-nf-regular.woff,meslolgs-nf-bold.woff,meslolgs-nf-italic.woff,meslolgs-nf-bold-italic.woff}" && \
    CODE_WORKBENCH="$(find /usr/lib/code-server -name "*workbench.html")" && \
    sed -i "s|</head>|\
    <style> \n\
        @font-face { \n\
        font-family: 'MesloLGS NF'; \n\
        font-style: normal; \n\
        src: url('_static/src/browser/pages/meslolgs-nf-regular.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-bold.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-italic.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-bold-italic.woff') format('woff'); \n\
    } \n\
    \n\</style></head>|g" "$CODE_WORKBENCH"

# install oh-my-zsh
RUN echo [$ARCH] create home folder backup... && \
    tar -cvzpf /tmp/backup.tar.gz /home/podman && rm -rf /home/podman && \
    mkdir -p /home/podman/npm && chown podman:podman -R /home/podman
USER podman
RUN echo [$ARCH] install oh-my-zsh... && \
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended && \
    git clone https://github.com/Aloxaf/fzf-tab /home/podman/.oh-my-zsh/custom/plugins/fzf-tab && \
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/podman/.oh-my-zsh/custom/themes/powerlevel10k && \
    /home/podman/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install

USER root
RUN echo [$ARCH] restore home folder backup... && \
    tar -xvzpf /tmp/backup.tar.gz -C / && chown podman:podman -R /home/podman

USER podman

RUN echo [$ARCH] install nix... && \
    curl -L https://nixos.org/nix/install | sh -s -- --no-daemon
RUN echo [$ARCH] install maven... && \
    /home/podman/.nix-profile/bin/nix-env -iA maven -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN echo [$ARCH] jdk17 nodejs... && \
    /home/podman/.nix-profile/bin/nix-env -iA jdk17 -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN echo [$ARCH] install nodejs... && \
    /home/podman/.nix-profile/bin/nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz && \
    /home/podman/.nix-profile/bin/npm config set prefix "/home/podman/npm" && \
    echo [$ARCH] install jji... && \
    /home/podman/.nix-profile/bin/npm i -g jji

# setup vscode-server
# version can be checked here https://github.com/coder/code-server/releases
RUN echo [$ARCH] install code-server extensions... && \
    code-server --install-extension carlos-algms.make-task-provider && \
    code-server --install-extension ms-vscode.makefile-tools && \
    code-server --install-extension redhat.java && \
    code-server --install-extension vscjava.vscode-java-debug && \
    code-server --install-extension vscjava.vscode-java-dependency && \
    code-server --install-extension vscjava.vscode-java-pack && \
    code-server --install-extension vscjava.vscode-java-test && \
    code-server --install-extension vscjava.vscode-maven && \
    code-server --install-extension wmanth.jar-viewer && \
    code-server --install-extension KylinIDETeam.gitlens && \
    echo [$ARCH] finish install.sh...

USER root

VOLUME /home/podman/.local/share/containers

RUN chown podman:podman -R /home/podman

ENV _CONTAINERS_USERNS_CONFIGURED=""

STOPSIGNAL SIGRTMIN+3

WORKDIR /home/podman
ENTRYPOINT ["/etc/systemd/system/container-entrypoint.sh"]

