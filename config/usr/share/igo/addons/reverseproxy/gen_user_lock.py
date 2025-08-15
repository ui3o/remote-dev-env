import os
import re


# create default
ENV_LIST = ""
ENV_LIST_EXPORTABLE = ""
GLOBAL_PORT_START = 9000
USER_PORT_START = 11000
# get the max port counter from environment variable or use default

LOCAL_PORT_LIST = os.getenv(
    "ENV_PARAM_REVERSEPROXY_LOCAL_PORT_LIST",
    "ADMIN;CODE;RSH;LOCAL1;LOCAL2",
)

GLOBAL_PORT_LIST = os.getenv(
    "ENV_PARAM_REVERSEPROXY_GLOBAL_PORT_LIST",
    "GRAFANA;GLOBAL1;GLOBAL2",
)

DEDICATED_LOCAL_NAMES = LOCAL_PORT_LIST.split(";")
DEDICATED_LOCAL_NAMES = [arg.strip() for arg in DEDICATED_LOCAL_NAMES if arg]
DEDICATED_GLOBAL_NAMES = GLOBAL_PORT_LIST.split(";")
DEDICATED_GLOBAL_NAMES = [arg.strip() for arg in DEDICATED_GLOBAL_NAMES if arg]


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
    global ENV_LIST
    global ENV_LIST_EXPORTABLE
    paramList.append(f"--label {prefix}{name}Env={value}")
    if prefix:
        prefix = prefix + "_"
    paramList.append(f"-e {prefix.upper()}{name}={value}")
    ENV_LIST = ENV_LIST + f"{prefix.upper()}{name}={value}\\\n"
    # ENV_LIST_EXPORTABLE = (
    #     ENV_LIST_EXPORTABLE + f"export {prefix.upper()}{name}={value}\\n"
    # )
    pass


def createLabelList(user: str, email: str, portStart: int) -> str:
    paramList = []
    for idx, name in enumerate(DEDICATED_LOCAL_NAMES, start=0):
        count = portStart + idx
        appendLabel(paramList, "port", name, str(count))

    for idx, name in enumerate(DEDICATED_GLOBAL_NAMES, start=0):
        count = GLOBAL_PORT_START + idx
        appendLabel(paramList, "port", name, str(count))
    # unique list
    appendLabel(paramList, "", "DEVELOPER", user)
    appendLabel(paramList, "", "DEVELOPER_EMAIL", email)
    appendLabel(paramList, "", "USER", "root")
    appendLabel(paramList, "", "HOME", "/root")
    paramList.append(f"-e ENV_LIST={ENV_LIST}")
    # paramList.append(f"-e ENV_LIST_EXPORTABLE={ENV_LIST_EXPORTABLE}")
    return " ".join(paramList)
