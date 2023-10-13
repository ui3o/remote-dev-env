// ttyd daemon service
/// <reference path="../../../types.d.ts"/>

service.setup = {
    /** @param ss service tools */
    onStart: async (ss) => {
        // basic auth done by traefik or else
        ss.exec.uid('podman').gid('podman').wd("/home/podman")
            .do(`sudo`, `-u`, `podman`, '/bin/ttyd', '-W', '-I', '/etc/ttyd/inline.html', 'zsh');
    },

}