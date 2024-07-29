import datetime
import os
import subprocess
import sys
import threading
import re

class JobDefinition:
    Name : str
    SrcPath: str
    StartPath: str|list[str]
    StopPath:str
    Type: str
    Event: threading.Event
    IgnoreFirst: bool
    Enabled: bool
    Interval: float
    Retry: int
    User: str
    Group: str
    Proc: subprocess.Popen
    def __init__ (self, 
                  srcPath: str,
                  name : str,
                  startPath: str|list[str],
                  type: str,
                  event: threading.Event,
                  stopPath: str,
                  ignoreFirst: bool,
                  interval: float,
                  retry: int,
                  enabled: bool,
                  user:str,
                  group:str):
        self.SrcPath = srcPath
        self.Name = name
        self.StartPath = startPath
        self.StopPath = stopPath
        self.Type = type
        self.Event = event
        self.IgnoreFirst = ignoreFirst
        self.Interval = interval
        self.Retry = retry
        self.Enabled = enabled
        self.User = user
        self.Group = group

scheduledList:dict[str, JobDefinition] = {}
runningList:dict[str, JobDefinition] = {}

def msg(type:str, stage:str, cmd:str, msg:str, err:bool = False):
    _out = ">"
    if err is True:
        _out = "!"
    sys.stdout.write(f"[{datetime.datetime.now()}][{_out}][{type}][{stage}][{cmd}] {msg}\n")

def runShell(runJob:JobDefinition, isStop:bool = False):
    _path = runJob.StartPath
    _cmd = runJob.StartPath
    _stage = "start"
    if isStop == True:
        _stage = "stop"      
        _path = runJob.StopPath
        _cmd = runJob.StopPath
        if isinstance(runJob.StopPath, list):
            _path = runJob.StopPath[0]
    if isinstance(runJob.StartPath, list):
        _path = runJob.StartPath[0]
    _msgHeader = runJob.Type
    if not _cmd:
        return None
    try:
        msg("popen", _stage, _path, f"type: {isStop}, u/g:{runJob.User}/{runJob.Group}, start cmd:{_cmd}")
        p = subprocess.Popen(
            _cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            user=runJob.User,
            group=runJob.Group
        )
        runJob.Proc = p

        def escape_ansi(line:str):
            line = re.sub(r'[\n\r\t]',' ',line)
            ansi_escape = re.sub(r'(\x9B|\x1B\[)[0-?]*[ -\/]*[@-~]', '', line)
            return ansi_escape

        def readIo(fileno:int, err: bool = False):
            o = os.read(fileno, 10240)
            while p.poll() is None or o:
                if o:
                    line = o.decode("ascii")
                    result = escape_ansi(line)
                    msg(_msgHeader, _stage, _path, result, err)
                o = os.read(fileno, 10240)
                
        th = threading.Thread(target=readIo,args=[p.stdout.fileno()])
        th.daemon = True
        th.start()
        th = threading.Thread(target=readIo, args=[p.stderr.fileno(), True])
        th.daemon = True
        th.start()
        while p.poll() is None:
            runJob.Event.wait(0.1)
        return p.returncode
    except FileNotFoundError:
        return None


def threadJob(
    runJob:JobDefinition
):
    while not runJob.Event.is_set() and runJob.Retry > 0:
        runShell(runJob)
        if runJob.Type != "interval":
            runJob.Retry = runJob.Retry - 1
        runJob.Event.wait(runJob.Interval)
    runJob.Event.is_set()


def scheduleAll():
    global scheduledList
    for k in list(runningList.keys()):
        if scheduledList.get(k, None) is None:
            # stop running job because it was removed from config file
            runJob = runningList.pop(k, None)
            runJob.Proc.terminate()
            msg("stoptask", "stop", runJob.Name, "stop task start")
            runJob.Event.set()
            runShell(runJob, True)
    for k in list(scheduledList.keys()):
        if runningList.get(k, None) is None:
            runJob = scheduledList.pop(k, None)
            runningList[k] = runJob
            # start job because it is not scheduled yet
            th = threading.Thread(target=threadJob, args=[runJob])
            th.daemon = True
            th.start()
    scheduledList = {}


class Jobs:
    def createOneTime(self, name: str, startPath: str|list[str], stopPath: str = "", retry=1, enabled = True, group:str = None):
        user = os.environ["INITS_USER"]
        srcPath = os.environ["INITS_SRC_PATH"]
        _group = ""
        if group is not None:
            _group = group
        scheduledList[srcPath + name + str(startPath) + str(stopPath) + str(retry) + "ontime" + str(enabled) + _group] = JobDefinition(
            srcPath, name, startPath, "onetime", threading.Event(), stopPath, False, 0.05, retry, enabled, user, group
        )
        

    def createInterval(self, name: str, startPath: str|list[str], interval: int, stopPath: str = "", ignoreFirst=False, enabled = True, group:str = None):
        user = os.environ["INITS_USER"]
        srcPath = os.environ["INITS_SRC_PATH"]
        _group = ""
        if group is not None:
            _group = group
        scheduledList[srcPath + name + str(startPath) + str(stopPath) + str(interval) + "interval" + str(ignoreFirst) + str(enabled) + _group] = JobDefinition(
            srcPath, name, startPath, "interval", threading.Event(), stopPath, ignoreFirst, interval, 1, enabled, user, group
        )

jobs = Jobs()
