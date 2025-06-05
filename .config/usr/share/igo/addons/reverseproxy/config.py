conf = {
    "timer": 0,
    "start": {
        "restartCount": 0,
        "envs": {},
        "params": [
            "--server_cert=/usr/share/igo/addons/reverseproxy/example_cert/example_server_cert.pem"
            "--server_key=/usr/share/igo/addons/reverseproxy/example_cert/example_server_key.pem"
            "--simple_auth_template_path=/usr/share/igo/addons/reverseproxy/simple/index.html"
        ],
    },
    "stop": {
        "restartCount": 0,
        "envs": {},
        "params": [],
    },
}

print(conf)
