FROM docker.io/archlinux:latest

# ARGS
ARG ARCH=amd64

EXPOSE 8080

# install base packages
RUN pacman -Syu --noconfirm sudo fakeroot binutils rsync mandoc openssh ca-certificates gnupg net-tools git-lfs cmatrix cowsay htop sssd procps-ng ncdu xz nnn ranger wget zsh git neovim tmux fzf make tree unzip podman fuse-overlayfs

# add sudo privileges to podman
RUN echo "@includedir /etc/sudoers.d" >> /etc/sudoers
RUN echo "podman ALL=(ALL:ALL) NOPASSWD:ALL" >> /etc/sudoers.d/podman

COPY ./.config/user/ /home/podman/
COPY ./.config/etc/ /etc/
COPY ./.config/user/.gitconfig /root/.gitconfig
COPY ./.config/root/ /root/

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

# for precopied source
RUN echo [$ARCH] create home folder backup... && \
    tar -cvzpf /tmp/backup.tar.gz /home/podman && rm -rf /home/podman && \
    mkdir -p /home/podman/npm && chown podman:podman -R /home/podman

USER podman
# setup vscode-server
# version can be checked here https://github.com/coder/code-server/releases
RUN echo [$ARCH] setup vscode-server... && \
    curl -fsSL https://code-server.dev/install.sh | sh -s;
RUN echo [$ARCH] install oh-my-zsh... && \
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended && \
    git clone https://github.com/Aloxaf/fzf-tab /home/podman/.oh-my-zsh/custom/plugins/fzf-tab && \
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/podman/.oh-my-zsh/custom/themes/powerlevel10k && \
    /home/podman/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install
# setup vscode-server extensions
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
    echo [$ARCH] finish extension install.sh..

USER root
RUN rm -rf /home/podman/.cache/code-server;\
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

RUN echo [$ARCH] restore home folder backup... && \
    tar -xvzpf /tmp/backup.tar.gz -C / && chown podman:podman -R /home/podman

RUN echo [$ARCH] install nix... && \
    sh <(curl -L https://nixos.org/nix/install) --daemon
RUN echo [$ARCH] install maven... && \
    /root/.nix-profile/bin/nix-env -iA maven -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN echo [$ARCH] jdk17 nodejs... && \
    /root/.nix-profile/bin/nix-env -iA jdk17 -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN echo [$ARCH] install nodejs... && \
    /root/.nix-profile/bin/nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz
RUN /root/.nix-profile/bin/npm config set prefix "/home/podman/npm" && \
    echo [$ARCH] install jji... && \
    /root/.nix-profile/bin/npm i -g jji && \
    echo [$ARCH] install pol... && \
    /root/.nix-profile/bin/npm i -g process-list-manager && \
    echo [$ARCH] install pol npm packages... && \
    /root/.nix-profile/bin/npm --prefix /etc/pol install
# process-list-manager log setup
RUN mkdir /var/log/pol && chmod o+rwx /var/log/pol

USER podman

ENV PATH="/nix/var/nix/profiles/default/bin:$PATH"
ENV ZSH=/home/podman/.oh-my-zsh
RUN /home/podman/npm/bin/pol completion zsh

USER root

ENV _CONTAINERS_USERNS_CONFIGURED=""
ENV PATH="/root/.nix-profile/bin/:$PATH"

RUN sudo chmod 4755 /usr/bin/newgidmap /usr/bin/newuidmap
RUN chown podman:podman -R /home/podman

STOPSIGNAL SIGRTMIN+3
WORKDIR /home/podman

USER podman

ENTRYPOINT ["sudo","/etc/pol/container-entrypoint.sh"]
