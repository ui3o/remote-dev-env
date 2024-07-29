#!python3
# source: https://github.com/phusion/baseimage-docker/blob/rel-0.9.16/image/bin/my_init
import os
import os.path
import sys
import signal
import glob
import threading
import datetime
import baseconf
from os.path import isdir, join
from os import listdir

KILL_PROCESS_TIMEOUT = 10
terminated_child_processes = {}
PID = os.getpid()
ExitEvent = threading.Event()
OS_HOME = {
    "darwin": "/Users"
}
HOME = OS_HOME.get(sys.platform, "/home")

class AlarmException(Exception):
    pass


def msg(partMsg, message):
    sys.stderr.write(f"[{datetime.datetime.now()}][ {partMsg} ] {message}\n")


def raise_alarm_exception():
    if not ExitEvent.is_set():
        ExitEvent.set()
    raise AlarmException("Alarm")


# Waits for the child process with the given PID, while at the same time
# reaping any other child processes that have exited (e.g. adopted child
# processes that have terminated).
def waitpid_reap_other_children(pid):
    global terminated_child_processes

    status = terminated_child_processes.get(pid)
    if status:
        # A previous call to waitpid_reap_other_children(),
        # with an argument not equal to the current argument,
        # already waited for this process. Return the status
        # that was obtained back then.
        del terminated_child_processes[pid]
        return status

    status = None

    while not ExitEvent.is_set():
        try:
            this_pid, status = os.waitpid(-1, 0)
            msg("PID", "PID %d exited with status %d" % (this_pid, status))

            if this_pid == pid:
                ExitEvent.set()
            else:
                # Save status for later.
                terminated_child_processes[this_pid] = status
        except OSError:
            pass
        ExitEvent.wait(0.1)
    return status


def stop_child_process(signo=signal.SIGINT, time_limit=KILL_PROCESS_TIMEOUT):
    msg("INIT", "Shutting down inits (PID %d) with signal(%d)..." % (PID, signo))
    try:
        os.kill(PID, signo)
    except OSError:
        pass
    signal.alarm(time_limit)
    try:
        try:
            waitpid_reap_other_children(PID)
        except OSError:
            pass
    except AlarmException:
        msg(
            "PID",
            f"inits (PID {PID}) did not shut down in time. Forcing it to exit.",
        )
        try:
            os.kill(PID, signal.SIGKILL)
        except OSError:
            pass
        try:
            waitpid_reap_other_children(PID)
        except OSError:
            pass
    finally:
        signal.alarm(0)


def mainLoop():
    try:
        msg("INIT", "inits is started with PID %d ..." % PID)
        try:
            waitpid_reap_other_children(PID)
        except KeyboardInterrupt:
            stop_child_process()
            raise
        except BaseException as s:
            msg("INIT", "An error occurred. Aborting... %s" % s)
            stop_child_process()
            raise
        finally:
            pass
        exit(0)
    finally:
        pass


def cron():
    sys.stderr.write(f"[{datetime.datetime.now()}][ INIT ] job controller started.\n")
    while not ExitEvent.is_set():
        fileUser = {}
        files = glob.glob("/etc/inits/adhoc.py")
        files = files + glob.glob("/etc/inits/system.py")
        for file in files:
            fileUser[file] = "root"

        homes = [f for f in listdir(HOME) if isdir(join(HOME, f))]
        for userHome in homes:
            files = glob.glob(f"{HOME}/{userHome}/.config/inits/adhoc.py")
            files = files + glob.glob(f"{HOME}/{userHome}/.config/inits/system.py")
            for file in files:
                fileUser[file] = userHome
            
        for k in list(fileUser.keys()):
            os.environ["INITS_SRC_PATH"] = k
            os.environ["INITS_USER"] = fileUser.get(k, None)
            with open(k) as f:
                code = compile(f.read(), k, "exec")
                exec(code, globals(), locals())
        baseconf.scheduleAll()
        ExitEvent.wait(0.1)


def threadCreator(targets=[]):
    for t in targets:
        th = threading.Thread(target=t)
        th.daemon = True
        th.start()


# Run main function.
signal.signal(signal.SIGALRM, lambda signum, frame: raise_alarm_exception())
signal.signal(signal.SIGINT, lambda _signum, _frame: raise_alarm_exception())
try:
    threadCreator([mainLoop, cron])
    while not ExitEvent.is_set():
        ExitEvent.wait()
except AlarmException:
    pass
except KeyboardInterrupt:
    msg("INIT", "Init system aborted.")
    exit(2)
finally:
    pass
