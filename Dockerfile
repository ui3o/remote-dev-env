# stable/Dockerfile
#
# Build a Podman container image from the latest
# stable version of Podman on the Fedoras Updates System.
# https://bodhi.fedoraproject.org/updates/?search=podman
# This image can be used to create a secured container
# that runs safely with privileges within the container.
#
FROM registry.fedoraproject.org/fedora:latest

# default configs
ENV CODE_SERVER_VERSION="4.12.0"

EXPOSE 8080

# Don't include container-selinux and remove
# directories used by yum that are just taking
# up space.
RUN dnf -y update; yum -y reinstall shadow-utils; \
    yum -y install\
    man\
    openssh\
    openssh-clients\
    ca-certificates\
    gnupg\
    net-tools\
    git-lfs\
    cmatrix\
    cowsay\
    htop\
    ncdu\
    xz\
    ranger\
    wget\
    zsh\
    git\
    neovim\
    tmux\
    fzf\
    make\
    tree\
    unzip\
    podman\
    fuse-overlayfs --exclude container-selinux; \
    rm -rf /var/cache /var/log/dnf* /var/log/yum.*

# install systemd
RUN dnf -y install systemd 

RUN useradd podman; \
    echo podman:10000:5000 > /etc/subuid; \
    echo podman:10000:5000 > /etc/subgid;

# install vscode server
# version can be checked here https://github.com/coder/code-server/releases
RUN curl -fL https://github.com/coder/code-server/releases/download/v$CODE_SERVER_VERSION/code-server-$CODE_SERVER_VERSION-amd64.rpm -o /tmp/code-server.rpm;\
    rpm -i /tmp/code-server.rpm
COPY config.yaml /home/podman/.config/code-server/config.yaml
RUN mkdir -m 0755 /nix && chown podman /nix
USER podman
RUN code-server --install-extension carlos-algms.make-task-provider;\
    code-server --install-extension KylinIDETeam.gitlens;\
    code-server --install-extension ms-vscode.makefile-tools;\
    code-server --install-extension redhat.java;\
    code-server --install-extension vscjava.vscode-java-debug;\
    code-server --install-extension vscjava.vscode-java-dependency;\
    code-server --install-extension vscjava.vscode-java-pack;\
    code-server --install-extension vscjava.vscode-java-test;\
    code-server --install-extension vscjava.vscode-maven;\
    code-server --install-extension wmanth.jar-viewer

# git setup
COPY ./.config/.gitconfig /home/podman/.gitconfig
COPY ./.config/.gitconfig /root/.gitconfig
# install oh-my-zsh
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
RUN git clone https://github.com/Aloxaf/fzf-tab ~/.oh-my-zsh/custom/plugins/fzf-tab
# install powerlevel10k prompt
RUN git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k
RUN /home/podman/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install
COPY ./.config/.p10k.zsh /home/podman/.p10k.zsh
COPY ./.config/.oh-my-zsh /home/podman/.oh-my-zsh
COPY ./.config/.local /home/podman/.local
# install nix
RUN curl -L https://nixos.org/nix/install | sh -s -- --no-daemon
# install nodejs
RUN /home/podman/.nix-profile/bin/nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz
RUN /home/podman/.nix-profile/bin/npm config set prefix "/home/podman/npm"

RUN /home/podman/.nix-profile/bin/npm i -g jji

# install zshrc
COPY ./.config/.zshrc /home/podman/.zshrc
COPY ./.config/.zshenv /home/podman/.zshenv
# tmux setup
COPY .config/.tmux.conf /home/podman

USER root
# install MeslolGS font for vscode
RUN cd /usr/lib/code-server/src/browser/pages; \
    curl -O "https://demyx.sh/fonts/{meslolgs-nf-regular.woff,meslolgs-nf-bold.woff,meslolgs-nf-italic.woff,meslolgs-nf-bold-italic.woff}"; \
    CODE_WORKBENCH="$(find /usr/lib/code-server -name "*workbench.html")"; \
    sudo sed -i "s|</head>|\
    <style> \n\
        @font-face { \n\
        font-family: 'MesloLGS NF'; \n\
        font-style: normal; \n\
        src: url('_static/src/browser/pages/meslolgs-nf-regular.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-bold.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-italic.woff') format('woff'), \n\
        url('_static/src/browser/pages/meslolgs-nf-bold-italic.woff') format('woff'); \n\
    } \n\
    \n\</style></head>|g" "$CODE_WORKBENCH";

# add sudo privileges to podman
RUN echo "@includedir /etc/sudoers.d" >> /etc/sudoers
RUN echo "podman ALL=(ALL:ALL) NOPASSWD:ALL" >> /etc/sudoers.d/podman

# VOLUME /var/lib/containers
VOLUME /home/podman/.local/share/containers

ADD https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/containers.conf /etc/containers/containers.conf
ADD https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/podman-containers.conf /home/podman/.config/containers/containers.conf
# chmod containers.conf and adjust storage.conf to enable Fuse storage.
RUN chmod 644 /etc/containers/containers.conf;
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock; touch /var/lib/shared/vfs-images/images.lock; touch /var/lib/shared/vfs-layers/layers.lock

COPY ./.config/root/ /root/
RUN usermod --shell /usr/bin/zsh podman
RUN chown podman:podman -R /home/podman

ENV _CONTAINERS_USERNS_CONFIGURED=""

STOPSIGNAL SIGRTMIN+3

WORKDIR /home/podman
COPY ./.config/etc/ /etc/
ENTRYPOINT ["/etc/systemd/system/container-entrypoint.sh"]

