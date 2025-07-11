FROM docker.io/fedora:latest

# ARGS
ARG TARGETPLATFORM
RUN echo "export ARCH=$(echo $TARGETPLATFORM | { IFS=/ read _ ARCH _; echo $ARCH; })" >> /arch
EXPOSE 10123
EXPOSE 7681

# install base packages host
RUN dnf upgrade -y && dnf install -y dnf-plugins-core
RUN dnf copr enable -y varlad/zellij && dnf copr enable -y totalfreak/lazygit
RUN dnf install -y binutils rsync mandoc ncat \
    openssh sshd openssl ca-certificates gnupg1 net-tools git-lfs cmatrix cowsay \
    htop sssd procps-ng ncdu xz nnn ranger zsh git neovim tmux \
    fzf make tree unzip podman fuse-overlayfs less zellij ripgrep lazygit lsof golang \
    telnet iputils

RUN curl -L https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.$(arch) -o /opt/ttyd && \
    chmod +x /opt/ttyd

RUN curl -k https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.13.16/openshift-client-linux.tar.gz --output /tmp/oc.tar.gz && \
  tar -xf /tmp/oc.tar.gz -C /bin && rm /tmp/oc.tar.gz

# setup file system for podman
RUN . /arch;echo [$ARCH] setup file system for podman... && \
    mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers && \
    touch /var/lib/shared/overlay-images/images.lock && \
    touch /var/lib/shared/overlay-layers/layers.lock && \
    touch /var/lib/shared/vfs-images/images.lock && \
    touch /var/lib/shared/vfs-layers/layers.lock

# setup vscode-server
# version can be checked here https://github.com/coder/code-server/releases
RUN . /arch;echo [$ARCH] setup vscode-server ... && \
    curl -fsSL https://code-server.dev/install.sh | sh -s;
RUN . /arch; echo [$ARCH] install oh-my-zsh... && \
    sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended && \
    git clone https://github.com/Aloxaf/fzf-tab /root/.oh-my-zsh/custom/plugins/fzf-tab && \
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /root/.oh-my-zsh/custom/themes/powerlevel10k && \
    /root/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install
RUN curl -L https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.$(arch) -o /opt/ttyd && \
    chmod +x /opt/ttyd

RUN cd /usr/lib/code-server/src/browser/pages && \
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


STOPSIGNAL SIGRTMIN+3
ENV GOPROXY=https://proxy.golang.org,direct

# setup vscode-server extensions
RUN go install honnef.co/go/tools/cmd/staticcheck@latest
RUN go install honnef.co/go/tools/cmd/staticcheck@latest
RUN go install golang.org/x/tools/gopls@latest

ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
RUN . /arch;echo [$ARCH] install code-server extensions... && \
    code-server --install-extension golang.go && \
    code-server --install-extension ms-vscode.makefile-tools && \
#     code-server --install-extension redhat.java && \
#     code-server --install-extension vscjava.vscode-java-debug && \
#     code-server --install-extension vscjava.vscode-java-dependency && \
#     code-server --install-extension vscjava.vscode-java-pack && \
#     code-server --install-extension vscjava.vscode-java-test && \
#     code-server --install-extension vscjava.vscode-maven && \
#     code-server --install-extension wmanth.jar-viewer && \
#     code-server --install-extension KylinIDETeam.gitlens && \
    echo [$ARCH] finish extension install.sh..

COPY ./config/user/ /root/
VOLUME /var/lib/containers
ENV _CONTAINERS_USERNS_CONFIGURED=""

RUN chmod 4755 /usr/bin/newgidmap /usr/bin/newuidmap
# RUN chown -R podman:podman /home/podman && \
#     mkdir -p /home/podman/.local/share/containers

ENV PATH="/root/.local/bin:/root/go/bin:$PATH"

RUN usermod --shell /usr/bin/zsh root
WORKDIR /root
RUN git init && git add . && git commit -m "init" && \
    git remote add shared_dotfiles /shared/.dotfiles && \
    git remote add my_dotfiles ~/ss/.dotfiles


COPY ./config/etc/ /etc/
COPY ./config/usr /usr
RUN ln -s /etc/units /usr/share/igo/.runtime/units/system && \
    ln -sf /root/.config/units /usr/share/igo/.runtime/units/root

RUN ssh-keygen -A
WORKDIR /usr/share/igo/issh
RUN GOOS=windows GOARCH=amd64 go build -o bin/issh.exe . && \
    GOOS=darwin  GOARCH=amd64 go build -o bin/issh-mac-intel . && \
    GOOS=darwin  GOARCH=arm64 go build -o bin/issh-mac-arm . && \
    GOOS=linux   GOARCH=amd64 go build -o bin/issh-linux 

WORKDIR /usr/share/igo/igo
RUN GOOS=linux go build -o igo .
WORKDIR /usr/share/igo/addons/reverseproxy
RUN GOOS=linux go build -o reverseproxy.disabled .
WORKDIR /usr/share/igo/addons/admin
RUN GOOS=linux go build -o admin.disabled .
WORKDIR /usr/share/igo

ENV GIN_MODE=release
ENV ENV_PARAM_REVERSEPROXY_SERVER_CERT=/usr/share/igo/addons/reverseproxy/example_cert/example_server_cert.pem
ENV ENV_PARAM_REVERSEPROXY_SERVER_KEY=/usr/share/igo/addons/reverseproxy/example_cert/example_server_key.pem
ENV ENV_PARAM_REVERSEPROXY_TEMPLATE_ROOT_PATH=/usr/share/igo/addons/reverseproxy/

RUN	groupadd -f igo
ENTRYPOINT [ "/etc/entrypoint.sh" ]
