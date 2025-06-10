# dynamic environment variables
# source $USERPROFILE/.config/wsl-initializer/.configrc
export JAVA_HOME="${$(readlink -e $(type -p java))%*/bin/java}"
