// code-server service
/// <reference path="../../../types.d.ts"/>

service.setup = {
    /** @param ss service tools */
    onStart: async (ss) => {
        const privatePass = await ss.cli.splitByLine.do('cat', '/run/secrets/pp.store');
        process.env.PASSWORD = !privatePass.c ? privatePass.o[0] : 'podman';
        process.env.SHELL = '/bin/zsh';
        ss.exec.uid('podman').gid('podman').wd("/home/podman")
            .do(`sudo`, '-E', `-u`, `podman`, '/usr/bin/code-server');
    },

}