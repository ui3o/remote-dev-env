#!/bin/bash
set -e

container=podman
export container

if [ $# -eq 0 ]; then
	echo "bash" >>/etc/boot.sh
else
	quoted_args="$(printf " %q" "${@}")"
	echo "${quoted_args}" >>/etc/boot.sh
fi

export HOME="/home/podman"
env >>/etc/boot.env
export HOME="/root"

systemctl mask systemd-journald-audit.socket sys-kernel-config.mount sys-kernel-debug.mount sys-kernel-tracing.mount systemd-firstboot.service systemd-udevd.service systemd-modules-load.service
systemctl unmask systemd-logind
systemctl enable container-entrypoint.service

systemd_args="false --unit=container-entrypoint.target"
echo "$0: starting /lib/systemd/systemd $systemd_args"
exec /lib/systemd/systemd $systemd_args
