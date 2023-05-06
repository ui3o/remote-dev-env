#!/bin/bash

set -e

UNAME=$(uname -m)
if [[ "$UNAME" == *"arm"* || "$UNAME" == *"aarch"* ]]; then
	ARCH="arm64"
fi

echo start install on $ARCH architecture...

echo start install dnf packages...
dnf -y update
dnf -y reinstall shadow-utils
dnf -y install man openssh openssh-clients ca-certificates gnupg net-tools git-lfs cmatrix cowsay htop sssd sssd-tools procps-ng ncdu xz ranger wget zsh git neovim tmux fzf make tree unzip systemd podman fuse-overlayfs --exclude container-selinux
rm -rf /var/cache /var/log/dnf* /var/log/yum.*

# setup podman user
echo setup podman user...
sudo useradd podman
sudo echo podman:10000:5000 >/etc/subuid
sudo echo podman:10000:5000 >/etc/subgid
sudo usermod --shell /usr/bin/zsh podman

# setup vscode-server
# version can be checked here https://github.com/coder/code-server/releases
echo setup vscode-server...
curl -fL https://github.com/coder/code-server/releases/download/v$CODE_SERVER_VERSION/code-server-$CODE_SERVER_VERSION-$ARCH.rpm -o /tmp/code-server.rpm
rpm -i /tmp/code-server.rpm
code-server --install-extension carlos-algms.make-task-provider
code-server --install-extension KylinIDETeam.gitlens
code-server --install-extension ms-vscode.makefile-tools
code-server --install-extension redhat.java
code-server --install-extension vscjava.vscode-java-debug
code-server --install-extension vscjava.vscode-java-dependency
code-server --install-extension vscjava.vscode-java-pack
code-server --install-extension vscjava.vscode-java-test
code-server --install-extension vscjava.vscode-maven
code-server --install-extension wmanth.jar-viewer
# install MeslolGS font for vscode
cd /usr/lib/code-server/src/browser/pages
sudo curl -O "https://demyx.sh/fonts/{meslolgs-nf-regular.woff,meslolgs-nf-bold.woff,meslolgs-nf-italic.woff,meslolgs-nf-bold-italic.woff}"
CODE_WORKBENCH="$(find /usr/lib/code-server -name "*workbench.html")"
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
\n\</style></head>|g" "$CODE_WORKBENCH"

# install oh-my-zsh
echo install oh-my-zsh...
sudo su - podman sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
sudo su - podman git clone https://github.com/Aloxaf/fzf-tab ~/.oh-my-zsh/custom/plugins/fzf-tab
# install powerlevel10k prompt
sudo su - podman git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k
sudo su - podman /home/podman/.oh-my-zsh/custom/themes/powerlevel10k/gitstatus/install

# install nix
echo install nix and nodejs...
sudo su - podman curl -L https://nixos.org/nix/install | sh -s -- --no-daemon
sudo mkdir -m 0755 /nix
sudo chown podman /nix
# install nodejs
/sudo su - podman home/podman/.nix-profile/bin/nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz
/sudo su - podman home/podman/.nix-profile/bin/npm config set prefix "/home/podman/npm"
# install jji
/sudo su - podman home/podman/.nix-profile/bin/npm i -g jji

# setup file system for podman
echo setup file system for podman...
sudo curl -fL https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/containers.conf -o /etc/containers/containers.conf
curl -fL https://raw.githubusercontent.com/containers/libpod/master/contrib/podmanimage/stable/podman-containers.conf -o /home/podman/.config/containers/containers.conf
# chmod containers.conf and adjust storage.conf to enable Fuse storage.
sudo chmod 644 /etc/containers/containers.conf
sudo mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers
sudo touch /var/lib/shared/overlay-images/images.lock
sudo touch /var/lib/shared/overlay-layers/layers.lock
sudo touch /var/lib/shared/vfs-images/images.lock
sudo touch /var/lib/shared/vfs-layers/layers.lock
