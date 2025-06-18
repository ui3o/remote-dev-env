#!/bin/bash

USER=$1

echo "execute user.login.sh"

if [[ ! -d /home/$USER ]]; then
    echo "User folder /home/$USER does not exist. Create a user!"
    mkdir -p /usr/share/igo/.runtime/units /tmp/.logins/
    useradd $USER
    usermod -aG igo $USER
    usermod -aG igorun $USER
    usermod --shell /usr/bin/zsh $USER
    echo $USER:10000:5000 >/etc/subuid
    echo $USER:10000:5000 >/etc/subgid
    cp -a /home/__example__/. /home/$USER/
    chmod -R go-rwx /home/$USER/
    chown -R $USER:$USER /home/$USER/
fi

# /tmp/.logins/user not exist
if [[ ! -d /tmp/.logins/$USER ]]; then
    python3 /etc/units/user_login_lock_create.py /tmp/.logins/$USER
    ln -sf /home/$USER/.config/units/ /usr/share/igo/.runtime/units/$USER
    chown -h $USER:$USER /usr/share/igo/.runtime/units/$USER
fi
