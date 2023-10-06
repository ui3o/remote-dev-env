// this is the code service
/// <reference path="../../../types.d.ts"/>

service.setup = {
    /** @param ss service tools */
    onStart: async (ss) => {
        ss.exec.do(`nix-daemon`);
    },

}