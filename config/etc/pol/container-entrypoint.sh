#!/bin/bash
set -e

container=podman
export container

# if [ $# -eq 0 ]; then
# 	echo "bash" >>/etc/root.sh
# else
# 	quoted_args="$(printf " %q" "${@}")"
# 	echo "${quoted_args}" >>/etc/root.sh
# fi

HOME="/home/podman" env >>/etc/user.env
env >>/etc/root.env

echo initializing...
# podman fix
mount --make-rshared /
mkdir -p /var/lib/shared-containers/overlay /var/lib/shared-containers/overlay-images /var/lib/shared-containers/overlay-layers
git config --global user.name "$DEV_CONT_COMITTER_NAME"
git config --global user.email "$DEV_CONT_COMITTER_EMAIL"
chown podman:podman /home/podman/.gitconfig && rm -f /root/.gitconfig && cp /home/podman/.gitconfig /root/.gitconfig 

exec /home/podman/npm/bin/pol_init /home/podman/npm/bin/pol boot
