conf = {
    "timer": 3,
    "start": {
        "restartCount": 0,
        "wd": "/home/podman",
        "envs": {"DEBUGENV": "debugenv", "DEBUGEN": "debugenv"},
        "params": ["-message=Start addon 2", "-exitcode=0", "param3"],
    },
    "stop": {
        "restartCount": 0,
        "envs": {"DEBUGENV": "debugenv", "DEBUGEN": "debugenv"},
        "params": ["-message=Stop addon 2", "-exitcode=0", "-sleep=1", "param3"],
    },
}

print(conf)
