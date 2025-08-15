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

/shared/.config/layers/install.sh >> /tmp/boot.log 2>&1 || true

mkdir -p /usr/share/igo/.runtime/units /tmp/.runtime/logins /run/secrets/runtime
echo root:10000:5000 >/etc/subuid
echo root:10000:5000 >/etc/subgid

# git -C /root fetch --all >> /tmp/boot.log 2>&1 || true
# git -C /root merge --allow-unrelated-histories --no-edit  -Xtheirs shared_layers/master >> /tmp/boot.log 2>&1 || true
# git merge --allow-unrelated-histories --no-edit my_layers/master

echo "# container specific environment variable list" >> /root/.ssh/environment
awk -v t="$ENV_LIST" 'BEGIN{print t}' >> /root/.ssh/environment
echo PATH=$PATH >> /root/.ssh/environment

# check DEV_CONT_MODE_DISABLE_UNITS
if [[ "${DEV_CONT_MODE_DISABLE_UNITS:-false}" == "true" ]]; then
    echo "DEV_CONT_MODE_DISABLE_UNITS is set, remove units"
    rm /usr/share/igo/.runtime/units/root
fi

if [[ -n "${DEV_CONT_ENABLED_ADDONS_LIST:-}" ]]; then
    IFS=',' read -ra ADDONS <<< "$DEV_CONT_ENABLED_ADDONS_LIST"
    for addon in "${ADDONS[@]}"; do
        echo "Enable addon: $addon"
        mv /usr/share/igo/addons/$addon/$addon.disabled /usr/share/igo/addons/$addon/$addon.start || true
    done
fi

exec /usr/share/igo/igo/igo $@
