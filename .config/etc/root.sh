#!/bin/bash
# boot process always run as root
echo initializing...
# podman fix
mount --make-rshared /
chown -R podman:podman /home/podman/.local/share/containers/
git config --global user.name "$DEV_CONT_COMITTER_NAME"
git config --global user.email "$DEV_CONT_COMITTER_EMAIL"
cp /root/.gitconfig /home/podman/.gitconfig && chown podman:podman /home/podman/.gitconfig

# below lines are auto filled
