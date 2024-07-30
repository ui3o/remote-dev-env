FROM docker.io/fedora:latest

# ARGS
ARG TARGETPLATFORM
RUN echo "export ARCH=$(echo $TARGETPLATFORM | { IFS=/ read _ ARCH _; echo $ARCH; })" >> /arch
EXPOSE 8080

# install base packages
RUN dnf upgrade -y && dnf install -y dnf-plugins-core
RUN dnf copr enable -y varlad/zellij && dnf copr enable -y totalfreak/lazygit
RUN dnf install -y binutils rsync mandoc \
    openssh ca-certificates gnupg1 net-tools git-lfs cmatrix cowsay \
    htop sssd procps-ng ncdu xz nnn ranger wget zsh git neovim tmux \
    fzf make tree unzip podman fuse-overlayfs less zellij ripgrep lazygit lsof

RUN wget https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.x86_64 -O /opt/ttyd && \
    chmod +x /opt/ttyd

 RUN curl -k https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.13.16/openshift-client-linux.tar.gz --output /tmp/oc.tar.gz && \
  tar -xf /tmp/oc.tar.gz -C /bin && rm /tmp/oc.tar.gz

COPY ./.config/etc/ /etc/
COPY ./.config/user/.gitconfig /root/.gitconfig
COPY ./.config/root/ /root/

# setup podman user
RUN . /arch;echo start install on $ARCH architecture... && \
    echo [$ARCH] setup podman user... && \
    useradd podman && \
    echo podman:10000:5000 >/etc/subuid && \
    echo podman:10000:5000 >/etc/subgid && \
    usermod --shell /usr/bin/zsh podman

# setup file system for podman
RUN . /arch;echo [$ARCH] setup file system for podman... && \
    mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers && \
    touch /var/lib/shared/overlay-images/images.lock && \
    touch /var/lib/shared/overlay-layers/layers.lock && \
    touch /var/lib/shared/vfs-images/images.lock && \
    touch /var/lib/shared/vfs-layers/layers.lock

# for precopied source
RUN . /arch;echo [$ARCH] create npm folder... && \
    mkdir -p /home/podman/npm && chown podman:podman -R /home/podman

# setup vscode-server
# version can be checked here https://github.com/coder/code-server/releases
RUN . /arch;echo [$ARCH] setup vscode-server ... && \
    curl -fsSL https://code-server.dev/install.sh | sh -s;
RUN . /arch; echo [$ARCH] install oh-my-zsh... && \
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended && \
    git clone https://github.com/Aloxaf/fzf-tab /home/podman/.oh-my-zsh/custom/plugins/fzf-tab && \
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/podman/.oh-my-zsh/custom/themes/powerlevel10k && \
    /home/podman/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install
# setup vscode-server extensions
RUN . /arch;echo [$ARCH] install code-server extensions... && \
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

ENV NIX_CONFIG=$'filter-syscalls = false\nexperimental-features = nix-command flakes'
RUN . /arch;echo [$ARCH] install nix... && \
    sh <(curl -L https://nixos.org/nix/install) --daemon
RUN . /arch;echo [$ARCH] install maven... && \
    /root/.nix-profile/bin/nix-env -iA maven -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN . /arch;echo [$ARCH] install gradle... && \
    /root/.nix-profile/bin/nix-env -iA gradle -f https://github.com/NixOS/nixpkgs/archive/9957cd48326fe8dbd52fdc50dd2502307f188b0d.tar.gz
RUN . /arch;echo [$ARCH] jdk17 nodejs... && \
    /root/.nix-profile/bin/nix-env -iA jdk17 -f https://github.com/NixOS/nixpkgs/archive/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8.tar.gz
RUN . /arch;echo [$ARCH] install nodejs... && \
    /root/.nix-profile/bin/nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz
RUN . /arch;/root/.nix-profile/bin/npm config set prefix "/home/podman/npm" && \
    echo [$ARCH] install jji... && \
    /root/.nix-profile/bin/npm i -g jji

# ENV PATH="/nix/var/nix/profiles/default/bin:$PATH"
# ENV ZSH=/home/podman/.oh-my-zsh

COPY ./.config/user/ /home/podman/

RUN rm -rf /home/podman/.local/share/containers
VOLUME /var/lib/containers

ENV _CONTAINERS_USERNS_CONFIGURED=""
ENV PATH="/home/podman/.local/bin:/root/.nix-profile/bin/:/home/podman/npm/bin:$PATH"

RUN ssh-keygen -A
RUN chmod 4755 /usr/bin/newgidmap /usr/bin/newuidmap
RUN chown -R podman:podman /home/podman && \
    mkdir -p /home/podman/.local/share/containers

STOPSIGNAL SIGRTMIN+3

WORKDIR /home/podman
ENV PYTHONPATH=/opt/python3/lib/
COPY .config/opt /opt
ENTRYPOINT [ "python3", "/opt/python3/bin/inits.py" ]
