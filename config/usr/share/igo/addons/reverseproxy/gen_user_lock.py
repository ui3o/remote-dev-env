import os
import re


# create default
GLOBAL_PORT_START = 9000
USER_PORT_START = 11000
MAX_PORT_COUNTER = 20
DEDICATED_LOCAL_NAMES = [
    "RSH",
    "CODE",
    *[f"LPR{i}" for i in range(1, MAX_PORT_COUNTER + 1)],
]
DEDICATED_GLOBAL_NAMES = [
    "GRAFANA",
    "KIBANA",
    *[f"GPR{i}" for i in range(1, MAX_PORT_COUNTER + 1)],
]


# recursively find file name in a specific folder and file name has to be regex match
def find_files_by_regex(folder_path, filename_pattern):
    matches = []
    pattern = re.compile(filename_pattern)
    for root, dirs, files in os.walk(folder_path):
        for file in files:
            if pattern.match(file):
                matches.append(os.path.join(root, file))
    return matches


# Replace "old_text" with "new_text" in a file
def find_and_replace_in_file(file_path, old_text, new_text, count):
    with open(file_path, "r") as file:
        content = file.read()
    content = content.replace(old_text, new_text, count=count)
    with open(file_path, "w") as file:
        file.write(content)


# create file and write string into it
def create_file_with_content(file_path, content):
    with open(file_path, "w") as f:
        f.write(content)


def appendLabel(paramList: list[str], prefix: str, name: str, value: str):
    paramList.append(f"--label {prefix}{name}Env={value}")
    if prefix:
        prefix = prefix + "_"
    paramList.append(f"-e {prefix.upper()}{name}={value}")
    pass


def createLabelList(user: str, portStart: int) -> str:
    paramList = []
    for idx, name in enumerate(DEDICATED_LOCAL_NAMES, start=0):
        count = portStart + idx
        appendLabel(paramList, "port", name, str(count))

    for idx, name in enumerate(DEDICATED_GLOBAL_NAMES, start=0):
        count = GLOBAL_PORT_START + idx
        appendLabel(paramList, "port", name, str(count))
    # unique list
    appendLabel(paramList, "", "DEVELOPER", user)
    appendLabel(paramList, "", "USER", "root")
    appendLabel(paramList, "", "HOME", "/root")
    return " ".join(paramList)
