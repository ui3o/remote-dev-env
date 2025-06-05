#!/bin/bash

USER=$1

create_user_properties() {
    local user=$1
    ls *.$user >/dev/null 2>&1
    if [[ $? -ne 0 ]]; then
        echo "create user prop for $user"
        for ((i=11000; i<=15000; i+=100)); do
            ls $i.* >/dev/null 2>&1
            if [[ $? -ne 0 ]]; then
                echo "create user env /tmp/.logins/$i.$user"
                touch /tmp/.logins/$i.$user
                # create local ports
                arr=(RSH CODE)
                for k in $(seq 1 98); do
                    arr+=("LPR$k")
                done
                for idx in "${!arr[@]}"; do
                    p=$((i + $idx))
                    echo "${arr[$idx]}=$p" >> /tmp/.logins/$i.$user
                    echo "${user}_${arr[$idx]}=$p" >> /tmp/.logins/$i.$user
                done
                # create global ports
                arr=(GRAFANA KIBANA)
                for k in $(seq 1 98); do
                    arr+=("GPR$k")
                done
                for idx in "${!arr[@]}"; do
                    p=$((9000 + $idx))
                    echo "${arr[$idx]}=$p" >> /tmp/.logins/$i.$user
                    echo "${user}_${arr[$idx]}=$p" >> /tmp/.logins/$i.$user
                done
                # unique list
                echo "USER=$user" >> /tmp/.logins/$i.$user
                echo "${user}_USER=$user" >> /tmp/.logins/$i.$user
                echo "HOME=/home/$user" >> /tmp/.logins/$i.$user
                echo "${user}_HOME=/home/$user" >> /tmp/.logins/$i.$user
                break
            fi
        done
    fi
    ln -sf /home/$user/.config/units/ /usr/share/igo/.runtime/units/$user
    chown -h $user:$user /usr/share/igo/.runtime/units/$user
}

if [[ ! -d /home/$USER ]]; then
    echo "User folder /home/$USER does not exist. Create a user!"
    mkdir -p /usr/share/igo/.runtime/units /tmp/.logins/
    useradd $USER
    usermod -aG igo $USER
    usermod -aG igorun $USER
    usermod --shell /usr/bin/zsh $USER
    echo $USER:10000:5000 >/etc/subuid
    echo $USER:10000:5000 >/etc/subgid
    cp -a /home/podman/. /home/$USER/
    chmod -R go-rwx /home/$USER/
    chown -R $USER:$USER /home/$USER/
fi

create_user_properties "$USER"
