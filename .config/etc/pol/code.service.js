// this is the code service
/// <reference path="../../../types.d.ts"/>

service.setup = {
    /** @param ss service tools */
    onStart: async (ss) => {
        ss.exec.uid('podman').gid('podman').wd("/home/podman").do(`sudo`, `-u`, `podman`, '/usr/bin/code-server');
    },

}