#!/bin/bash
set -e

container=podman
export container

if [ $# -eq 0 ]; then
	echo >&2 'ERROR: No command specified. You probably want to run `journalctl -f`, or maybe `bash`?'
	exit 1
fi

if [ ! -t 0 ]; then
	echo >&2 'ERROR: TTY needs to be enabled (`docker run -t ...`).'
	exit 1
fi

env >/etc/docker-entrypoint-env

quoted_args="$(printf " %q" "${@}")"
echo "${quoted_args}" >/etc/docker-entrypoint-cmd

systemctl mask systemd-journald-audit.socket sys-kernel-config.mount sys-kernel-debug.mount sys-kernel-tracing.mount systemd-firstboot.service systemd-udevd.service systemd-modules-load.service
systemctl unmask systemd-logind
systemctl enable docker-entrypoint.service

systemd_args="false --unit=docker-entrypoint.target"
echo "$0: starting /lib/systemd/systemd $systemd_args"
exec /lib/systemd/systemd $systemd_args
