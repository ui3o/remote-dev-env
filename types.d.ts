/**
 * **service** definition.
 * Inside service methods possible to use **ss** (service tools) helper functions.
 */
declare const service: I_SERVICE;

declare interface I_SERVICE {
    /**
     * **setup:** setup service properly. All possible fields can be accessed during
     * the definitions.
     */
    setup: {
        /**
         * **onStart:** `pol` daemon will call this method on service start.
         */
        onStart: (ss: I_SS_START) => Promise<void>

        /**
         * @optional
         * **onStop:** `pol` daemon will call this method on service stop. After this method call
         * `pol` daemon will send `kill` signal to the running processes.
         * 
         * `Without this` implementation `pol` daemon will send `kill` signal to the running processes.
         * 
         * @example ```
         * service.onStop(async (ss) => {
         *   ss.toLog('pre stop');
         * });
         * ```
         */
        onStop: (ss: I_SS_STOP) => void;

        /**
         * @optional
         * **onLogin:** `pol` daemon will call this method when login state reached. Basically this 
         * state normally the shell state in Linux system.
         * 
         * This state only reached if `login.service.target.js` linked to that service which implement *onLogin* method.
         * 
         * @returns string[] command's list @example ```['ls', '-al']```
         * @returns cmd object @example ```{cmd:['ls', '-al'], uid: 1000, gid:1000, it: true}```
         */
        onLogin: (ss: I_SS_START) => Promise<void>
        // string[] | {
        //     /**
        //      * command list in array
        //      * @example
        //      * cmd: ['ls', '-al']
        //      */
        //     cmd: string[],
        //     /**
        //      * user id
        //      * @example
        //      * uid: 1000
        //      */
        //     uid: string,
        //     /**
        //      * group id
        //      * @example
        //      * guid: 1000
        //      */
        //     gid: string,
        //     /**
        //      * interactive run
        //      * @example
        //      * it: true
        //      */
        //     it: boolean
        // }

    }
}

declare interface I_SS {

    /**
     * **send strings to log file:** send the given string parameters to log file.
     */
    toLog: (...msg: string[]) => void;

    /**
     * **interpreted shell script:** execute external **script** which is **interpreted** into **string**, inside spawn.
     * **cli** means start spawn, stdout and stderr will be parsed.
     *
     * finalize the script call the **do** function at *last* position.
     *
     */
    cli: I_CLI;

}

declare interface I_SS_START extends I_SS {

    /**
     * **exec shell script:** execute external **script** which is the **real shell** execution in **on_start** function. **All message** on
     * **stdout** and **stderr** are logged. This is **running** in the **background**. Possible to surround `await ss.exec.do()` method
     * with pre/post start functionality.
     *
     * finalize the script call the **do** function at *last* position.
     *
     */
    exec: I_EXEC;
}

declare interface I_SS_STOP extends I_SS {

    /**
     * **stop remaining scripts:** this method send kill to remaining **scripts**.
     */
    stopAll: () => Promise<any>;
}

declare interface I_EXEC {
    /**
     * **uid** means set the process user
     * @param uid number or string
     */
    uid(uid: string | number): I_EXEC;
    /**
     * **gid** means set the process group
     * @param gid number or string
     */
    gid(gid: string | number): I_EXEC;
    /**
     * **wd** means set the process working directory
     * @param wd path to working directory
     */
    wd(wd: string): I_EXEC;
    /**
     * **it** run exec command in interactive terminal
     */
    it: I_EXEC;
    /**
     * **do** means spawn the command
     * 
     * **String** items are **splitted** by **space**. If **no split** need add items inside an **array**.
     * 
     * **First item** need to be the **program** name.
     *
     * examples:
     * * `const {o, c} = await ss.cli.splitByLine.do('ls', '-al')`
     * * `const {o: files, c: result} = await ss.cli.splitByLine.do('ls', '-al')`
     * * `const {o, c} = await ss.cli.splitByLine.do("bash -c", ["echo hello"])`
     *
     * @param cmd string list the first parameter is the program name
     * @returns command result c(code) and o(output)
     */
    do(...cmd: string[]): Promise<number>;
}

declare interface I_CLI {
    /**
     * **uid** means set the process user
     * @param uid number or string
     */
    uid(uid: string | number): I_CLI;
    /**
     * **gid** means set the process group
     * @param gid number or string
     */
    gid(gid: string | number): I_CLI;
    /**
     * **wd** means set the process working directory
     * @param wd path to working directory
     */
    wd(wd: string): I_CLI;
    /**
     * **noErr** means the output will not contains stderr output
     */
    noErr: I_CLI;
    /**
     * **eol** set end of line characters
     *
     * **default**: '\n'
     * @param eol end of line characters
     */
    eol(eol: string): I_CLI;
    /**
     * **splitByLine** means the output will be split by line
     */
    splitByLine: I_CLI;
    /**
     * **splitAll** means the output will be split by line and split lines by [space || tab]
     */
    splitAll: I_CLI;
    /**
     * **do** means spawn the command
     * 
     * **String** items are **splitted** by **space**. If **no split** need add items inside an **array**.
     * 
     * **First item** need to be the **program** name.
     *
     * examples:
     * * `const {o, c} = await ss.cli.splitByLine.do('ls', '-al')`
     * * `const {o: files, c: result} = await ss.cli.splitByLine.do('ls', '-al')`
     * * `const {o, c} = await ss.cli.splitByLine.do("bash -c", ["echo hello"])`
     *
     * @param cmd string list the first parameter is the program name
     * @returns command result c(code) and o(output)
     */
    do(...cmd: string[]): Promise<{ o: string | Array<string> | Array<Array<string>>; c: number }>;
}

