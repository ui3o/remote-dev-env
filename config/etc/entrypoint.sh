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


exec /usr/share/igo/igo/igo
