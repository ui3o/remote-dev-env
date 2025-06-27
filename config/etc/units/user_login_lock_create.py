import os
import re
import sys
import subprocess

if len(sys.argv) < 2:
    print("Usage: python user_login_lock_create.py <folder_path>")
    sys.exit(1)

# collect user data
folder_path = sys.argv[1]
user = folder_path.split("/")[-1]

# check user exist
if os.path.exists(folder_path):
    print("Current user path exist!")
    sys.exit(1)

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

FILE_REPLACE_STRINGS = [["/root/.zshrc", "___USER___", user, 1]]


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


files = find_files_by_regex("/tmp/.runtime/logins", r".*\.lock$")
userCounter = USER_PORT_START
if len(files) > 0:
    for file in files:
        count = int(file.split("/")[-1].split(".")[0])
        if count > userCounter:
            userCounter = count
    userCounter += 100

# create folder if not exists
if not os.path.exists(folder_path):
    os.makedirs(folder_path)

# /tmp/.runtime/logins/user/1000.lock
create_file_with_content(f"{folder_path}/{userCounter}.lock", "")
envList = []
for idx, name in enumerate(DEDICATED_LOCAL_NAMES, start=0):
    count = userCounter + idx
    envList.append(f"PORT_{name}={count}")
    create_file_with_content(f"{folder_path}/{name}.port", str(count))

for idx, name in enumerate(DEDICATED_GLOBAL_NAMES, start=0):
    count = GLOBAL_PORT_START + idx
    envList.append(f"PORT_{name}={count}")
    create_file_with_content(f"{folder_path}/{name}.port", str(count))

# unique list
envList.append(f"USERNAME={user}")
envList.append("USER=root")
envList.append("HOME=/root")

create_file_with_content(f"{folder_path}/.env", "\n".join(envList))

for replace in FILE_REPLACE_STRINGS:
    find_and_replace_in_file(replace[0], replace[1], replace[2], replace[3])
