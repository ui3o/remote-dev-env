#!/bin/bash
set -e

# podman fix
mount --make-rshared /

# mkdir -p /var/lib/shared-containers/overlay /var/lib/shared-containers/overlay-images /var/lib/shared-containers/overlay-layers
# git config --global user.name "$DEV_CONT_COMITTER_NAME"
# git config --global user.email "$DEV_CONT_COMITTER_EMAIL"
# chown podman:podman /home/podman/.gitconfig && rm -f /root/.gitconfig && cp /home/podman/.gitconfig /root/.gitconfig

# echo image share fix pre run...
# podman ps
# [[ ! -L /var/lib/containers/storage/overlay && -d /var/lib/containers/storage/overlay ]] && rm -rf /var/lib/containers/storage/overlay && ln -sf /var/lib/shared-containers/overlay /var/lib/containers/storage/overlay
# [[ ! -L /var/lib/containers/storage/overlay-images && -d /var/lib/containers/storage/overlay-images ]] && rm -rf /var/lib/containers/storage/overlay-images && ln -sf /var/lib/shared-containers/overlay-images /var/lib/containers/storage/overlay-images
# [[ ! -L /var/lib/containers/storage/overlay-layers && -d /var/lib/containers/storage/overlay-layers ]] && rm -rf /var/lib/containers/storage/overlay-layers && ln -sf /var/lib/shared-containers/overlay-layers /var/lib/containers/storage/overlay-layers

# set DEVELOPER podman run 

mkdir -p /usr/share/igo/.runtime/units /tmp/.runtime/logins /run/secrets/runtime
echo root:10000:5000 >/etc/subuid
echo root:10000:5000 >/etc/subgid

# check DEV_CONT_MODE_REVERSEPROXY_ONLY
if [[ "${DEV_CONT_MODE_REVERSEPROXY_ONLY:-false}" == "true" ]]; then
    echo "DEV_CONT_MODE_REVERSEPROXY_ONLY is set, remove units"
else
    rm -rf /tmp/.runtime/logins/$DEVELOPER
    python3 /etc/units/user_login_lock_create.py /tmp/.runtime/logins/$DEVELOPER
    mkdir -p /tmp/.runtime/logins/$DEVELOPER
    cp -rf /run/secrets/runtime/. /tmp/.runtime/logins/$DEVELOPER
    ln -sf /root/.config/units /usr/share/igo/.runtime/units/root
fi

# check DEV_CONT_MODE_NO_REVERSEPROXY
if [[ "${DEV_CONT_MODE_NO_REVERSEPROXY:-false}" == "true" ]]; then
    echo "DEV_CONT_MODE_NO_REVERSEPROXY is set, skipping reverse proxy setup"
    mv /usr/share/igo/addons/reverseproxy/reverseproxy.start /usr/share/igo/addons/reverseproxy/reverseproxy.disabled
fi

exec /usr/share/igo/igo/igo $@
