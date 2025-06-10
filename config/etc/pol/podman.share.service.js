// podman image share service
/// <reference path="../../../types.d.ts"/>

service.setup = {
    /** @param ss service tools */
    onStart: async (ss) => {
        ss.exec
            .do(`/etc/pol/podman.image.share.sh`);
    },

}
