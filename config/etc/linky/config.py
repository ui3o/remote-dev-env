# Mine default
# First entry is the rootpath to the layer.
# Mine default
# First entry is the rootpath to the layer.
layers = [
    # Root path of the layer
    "/mine/.layers",
    # soft sync layers to not overwrite all files under root dir
    # "-s /root",
    # hard sync overwrite every file under /etc/ssl/certs dir removing files that would not conflict otherwise
    # "-h /etc/ssl/certs",
]

# __cleanup__ = [
#     # First entry of the array where to move the files for cleanup
#     "/tmp/cleanup",
#     # File paths to cleanup after creating symlinks
#     "/root/path/to/remove",
#     "/other/path/to/remove",
# ]
