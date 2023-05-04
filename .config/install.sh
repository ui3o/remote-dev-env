#!/bin/bash
# insider install.sh which will be updated every time

set -e

IFS=':' read -a fields <<<"$PATH"
for field in "${fields[@]}"; do
  if [[ $field =~ ^(/mnt/.*)/AppData/Local/.* ]]; then
    export USERPROFILE="${BASH_REMATCH[1]}"
    break
  fi
done
echo "## Setup USER in /etc/environment"
echo "export USER=\"$USER\"" | sudo tee /etc/environment
echo "export USERPROFILE=\"$USERPROFILE\"" | sudo tee -a /etc/environment

export WSLINITIALIZER="$USERPROFILE/.config/wsl-initializer" 

# TODO program reinstall
sudo cp $WSLINITIALIZER/.config/etc/apt/sources.list /etc/apt/sources.list

# register update jobs to lifecycle, clone to /opt
$WSLINITIALIZER/.config/etc/cron.twomin/ws-update.sh
$WSLINITIALIZER/.config/etc/cron.twomin/ootp-update.sh

# update or save AD user and pass
echo "export AD_USER=\"$AD_USER\"" | tee $WSLINITIALIZER/.configrc > /dev/null
echo "export AD_PASS=\"$AD_PASS\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null
echo "export AD_PASSWORD=\"$AD_PASSWORD\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null

echo "export WS_EMAIL=\"$WS_EMAIL\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null
echo "export WS_COMITTER=\"$WS_COMITTER\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null
echo "export WS_BITBUCKET_TOKEN=\"$WS_BITBUCKET_TOKEN\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null
echo "export WS_NEXUS_TOKEN=\"$WS_NEXUS_TOKEN\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null
echo "export USERPROFILE=\"$USERPROFILE\"" | tee -a $WSLINITIALIZER/.configrc > /dev/null

# setup backup partiton
unzip -o $WSLINITIALIZER/resources/wsl-initializer.zip -d $WSLINITIALIZER > /dev/null

## setup backup machine
if ! wsl.exe -d wsl-backup date >/dev/null; then
  echo "## Install backup machine"
  sudo chmod ugo+x $WSLINITIALIZER/wsl-backup.exe > /dev/null
  $WSLINITIALIZER/wsl-backup.exe install > /dev/null
fi

echo "## Setup boot service in cron"
sudo cp $WSLINITIALIZER/.config/boot /usr/sbin/
sudo cp $WSLINITIALIZER/.config/runall /usr/sbin/
# # add to cron and skip on duplication
sudo rm -rf /etc/cron.twomin
sudo cp -R $WSLINITIALIZER/.config/etc/cron.twomin /etc
echo "" | crontab -
twomin="/usr/sbin/runall /etc/cron.twomin"
# # run on every 2 minutes https://crontab.guru/every-2-minutes
job="*/2 * * * * $twomin"
cat <(fgrep -i -v "$twomin" <(crontab -l)) <(echo "$job") | crontab - > /dev/null
sudo touch /etc/vimrc

echo "## Setup backup"
mkdir -p /mnt/wsl/wsl-backup
wsl.exe -d wsl-backup -u root mount --bind / /mnt/wsl/wsl-backup/ > /dev/null
if [ ! -d ~/.oh-my-zsh ]; then
  sudo mv /home/$USER /home/$USER-$(date +%s)
  sudo mkdir -p -m 700 /home/$USER && sudo chown $USER:$USER /home/$USER
  sudo mkdir -p -m 700 /mnt/wsl/wsl-backup/home/$USER && sudo chown $USER:$USER /mnt/wsl/wsl-backup/home/$USER
  sudo mkdir -p /mnt/wsl/wsl-backup/nix && sudo chown $USER:root /mnt/wsl/wsl-backup/nix
  sudo mkdir -p -m 755 /nix && sudo chown $USER:root /nix
  # compile fstab
  # sed "s|USER|$USER|g" $WSLINITIALIZER/.config/etc/fstab.template > $WSLINITIALIZER/.config/etc/fstab
  # cp fstab and wsl.conf
  # sudo cp $WSLINITIALIZER/.config/etc/fstab /etc/fstab
  sudo cp $WSLINITIALIZER/.config/etc/wsl.conf /etc/wsl.conf
fi
sudo mount --bind /mnt/wsl/wsl-backup/home/$USER /home/$USER > /dev/null || true
sudo mount --bind /mnt/wsl/wsl-backup/nix /nix > /dev/null || true

echo "## disable ssh verify for git"
cp -f $WSLINITIALIZER/.config/.gitconfig ~/.gitconfig
git config --global user.name "$WS_COMITTER"
git config --global user.email "$WS_EMAIL"

echo "## change shell to boot"
sudo usermod --shell /usr/sbin/boot $USER

echo "## install proxy certificate"
sudo cp -f $WSLINITIALIZER/.config/otp_ca2.crt /usr/local/share/ca-certificates
sudo cp -f $WSLINITIALIZER/.config/otp_ca10.crt /usr/local/share/ca-certificates
sudo cp -f $WSLINITIALIZER/.config/otp_operativ_ca10.crt /usr/local/share/ca-certificates
sudo cp -f $WSLINITIALIZER/.config/otp_proxy.crt /usr/local/share/ca-certificates
sudo update-ca-certificates

if [ ! -d ~/.oh-my-zsh ]; then
  echo "## install oh-my-zsh"
  rm -rf ~/.oh-my-zsh
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
  git clone https://github.com/Aloxaf/fzf-tab ~/.oh-my-zsh/custom/plugins/fzf-tab
fi

if [ ! -d ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k ]; then
  echo "## install powerlevel10k prompt"
  git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/themes/powerlevel10k
fi
cp -f $WSLINITIALIZER/.config/.p10k.zsh ~

echo "echo \"Acquire::http::proxy  \\\"http://\$AD_USER:\$AD_PASS@10.42.25.62:8080/\\\";\" | sudo tee /etc/apt/apt.conf > /dev/null" > /dev/null
echo "echo \"Acquire::ftp::proxy \\\"http://\$AD_USER:\$AD_PASS@10.42.25.62:8080/\\\";\" | sudo tee -a /etc/apt/apt.conf > /dev/null" > /dev/null
echo "echo \"Acquire::https::proxy \\\"http://\$AD_USER:\$AD_PASS@10.42.25.62:8080/\\\";\" | sudo tee -a /etc/apt/apt.conf > /dev/null" > /dev/null

echo "nameserver 192.168.200.99" | sudo tee /etc/resolv.conf > /dev/null
echo "nameserver 10.46.1.29" | sudo tee -a /etc/resolv.conf > /dev/null
echo "nameserver 8.8.8.8" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search corp.otpbank.hu" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search kozpont.otp" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search irfi.otp" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search fiok.otp" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search otp" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search nruo" | sudo tee -a /etc/resolv.conf > /dev/null
echo "search otpbank.hu" | sudo tee -a /etc/resolv.conf > /dev/null

echo "## init ssh-key for bitbucket"
mkdir -p $WSLINITIALIZER/.store/.ssh
if [ ! -f $WSLINITIALIZER/.store/.ssh/id_rsa ]; then
  echo "[INFO] You can copy your id_rsa and id_rsa.pub to $WSLINITIALIZER/.store/.ssh folder or generate one!"
  read -p "[QUESTION] Do you want me to generate id_rsa and id_rsa.pub? (y=generate/n=you are done with the copy) [y/N] " ready
  [ "$ready" = "y" ] && ssh-keygen -t rsa -f $WSLINITIALIZER/.store/.ssh/id_rsa -q -P ""
fi
if [ ! -f $WSLINITIALIZER/.store/.ssh/id_rsa ]; then
  echo "[ERROR] You did not copy your id_rsa to $WSLINITIALIZER/.store/.ssh folder!"
  echo "[ERROR] Copy or generate! Please run \`install.sh again\`!"
  exit 2
fi
if [ ! -f $WSLINITIALIZER/.store/.ssh/id_rsa.pub ]; then
  echo "[ERROR] You did not copy your id_rsa.pub to $WSLINITIALIZER/.store/.ssh folder!"
  echo "[ERROR] Copy or generate! Please run \`install.sh again\`!"
  exit 2
fi

# fix id_rsa user access
mkdir ~/.ssh && chmod 700 ~/.ssh
cp $WSLINITIALIZER/.store/.ssh/id_rsa ~/.ssh/id_rsa
cp $WSLINITIALIZER/.store/.ssh/id_rsa.pub ~/.ssh/id_rsa.pub
chmod 600 ~/.ssh/id_rsa
chmod 644 ~/.ssh/id_rsa.pub

echo "## install podman properties"
sudo mkdir -p /etc/containers
sudo mount --make-rshared /
echo -e "[registries.search]\nregistries = ['docker.io']" | sudo tee /etc/containers/registries.conf

if [ ! -d /nix/store ]; then
  echo "## install nix package manager"
  curl -L https://nixos.org/nix/install | sh -s -- --no-daemon
  source /home/$USER/.nix-profile/etc/profile.d/nix.sh
else
  source /home/$USER/.nix-profile/etc/profile.d/nix.sh
fi
echo "## install nodejs by nix package manager (it takes time)"
nix-env -iA nodejs-16_x -f https://github.com/NixOS/nixpkgs/archive/5e15d5da4abb74f0dd76967044735c70e94c5af1.tar.gz

# setup .config/.zshrc
echo "## setup .zshrc file"
cp -f $WSLINITIALIZER/.config/.zshrc ~/.zshrc
cp -f $WSLINITIALIZER/.config/.zshenv ~/.zshenv
cp -f $WSLINITIALIZER/.config/otp.zsh-theme ~/.oh-my-zsh/custom/themes/otp.zsh-theme

# setup .zshrc AD user
sed -i "s|export AD_USER=.*|export AD_USER=\"$AD_USER\"|g" ~/.zshrc
sed -i "s|export AD_PASS=.*|export AD_PASS=\"$AD_PASS\"|g" ~/.zshrc

# install jji
if [ ! -f ~/.npmrc ]; then
  echo "## setup npm"
  mkdir -p ~/npm
  npm config set strict-ssl false
  npm config set prefix "/home/$USER/npm"
  AUTH_TOKEN=$(echo -n "$AD_USER:$AD_PASSWORD" | openssl base64)
  echo "## AUTH_TOKEN=$AUTH_TOKEN"
  echo "@dbt:registry=https://otpnexus.hu/repository/dbt-ptr-npm-group/" | tee -a ~/.npmrc
  echo "//otpnexus.hu/repository/dbt-ptr-npm-group/:_auth=$AUTH_TOKEN" | tee -a ~/.npmrc
fi
npm cache clean --force
npm i -g jji

# Update/reinstall all old program
# node $WSLINITIALIZER/.config/debrefresh.js


if [ ! -f ~/.m2/settings.xml ]; then
  mkdir -p ~/.m2
  cp -f $WSLINITIALIZER/.config/.m2/settings.xml ~/.m2/settings.xml
fi

[ ! -f ~/.config/.zshrc ] && echo "## Setup custom ~/.config/.zshrc" && mkdir -p ~/.config && cp -rf $WSLINITIALIZER/.config/.zshrc.custom ~/.config/.zshrc

# java cacerts
mkdir -p ~/.config/java/
echo "## Setup ~/.config/java/cacerts"
cp -rf $WSLINITIALIZER/.config/java/cacerts ~/.config/java/cacerts

# openshift cli install
export REMOTE_VERSION=$(grep '"oc_version": "' $WSLINITIALIZER/package.json)
export LOCAL_VERSION=$(grep '"oc_version": "' /opt/wsl-initializer/package.json)
if [[ ! -f /bin/oc || "$REMOTE_VERSION" != "$LOCAL_VERSION" ]]; then
  echo "## install openshift cli $REMOTE_VERSION"
  curl https://downloads-openshift-console.apps.ocpcendv.ocp.otpbank.hu/amd64/linux/oc.tar --output /tmp/oc.tar
  sudo tar -xf /tmp/oc.tar -C /bin
fi

# set iptables to iptables-legacy
echo "## set iptables to iptables-legacy"
sudo update-alternatives --set iptables /usr/sbin/iptables-legacy
sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy

# set iptables to iptables-legacy
echo "## make plugin for .oh-my-zsh"
rm -rf ~/.oh-my-zsh/custom/plugins/make
cp -r $WSLINITIALIZER/.config/.oh-my-zsh/custom/plugins/make ~/.oh-my-zsh/custom/plugins

# Have to be last, if no access to repo, all install need to be done
npm i -g @dbt/ootp
