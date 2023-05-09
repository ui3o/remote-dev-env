#!/bin/bash
set -e

container=podman
export container

if [ $# -eq 0 ]; then
	echo "bash" >>/etc/root.sh
else
	quoted_args="$(printf " %q" "${@}")"
	echo "${quoted_args}" >>/etc/root.sh
fi

HOME="/home/podman" env >>/etc/user.env
env >>/etc/root.env

systemctl mask systemd-journald-audit.socket sys-kernel-config.mount sys-kernel-debug.mount sys-kernel-tracing.mount systemd-firstboot.service systemd-udevd.service systemd-modules-load.service
systemctl unmask systemd-logind
systemctl enable container-entrypoint.service

systemd_args="false --unit=container-entrypoint.target"
echo "$0: starting /lib/systemd/systemd $systemd_args"
exec /lib/systemd/systemd $systemd_args
