#!/bin/bash

USER=$1

rm -f /tmp/.logins/$USER
rm /usr/share/igo/.runtime/units/$USER

