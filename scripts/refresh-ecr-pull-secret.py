#!/usr/bin/env python3
"""Refresh the short-lived ECR image pull Secret for EC2 Kubernetes or OpenShift."""

import argparse
import base64
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository-url", required=True)
    parser.add_argument("--client", choices=("kubectl", "oc"), required=True)
    parser.add_argument("--namespace", default="firman-demo")
    parser.add_argument("--region", default="ap-southeast-3")
    args = parser.parse_args()

    match = re.fullmatch(
        r"([0-9]{12}\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com)/[a-z0-9/_-]+",
        args.repository_url,
    )
    if not match or match.group(2) != args.region:
        parser.error("repository-url must be an ECR repository in the selected region")

    password = subprocess.run(
        ["aws", "ecr", "get-login-password", "--region", args.region],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    if not password:
        raise SystemExit("AWS returned an empty ECR login password")

    registry = match.group(1)
    auth = base64.b64encode(f"AWS:{password}".encode()).decode()
    config = {"auths": {registry: {"username": "AWS", "password": password, "auth": auth}}}

    with tempfile.TemporaryDirectory(prefix="firman-ecr-") as directory:
        path = Path(directory) / "config.json"
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8") as file:
            json.dump(config, file)

        secret = subprocess.run(
            [
                args.client, "-n", args.namespace, "create", "secret", "generic", "ecr-pull",
                "--type=kubernetes.io/dockerconfigjson",
                f"--from-file=.dockerconfigjson={path}",
                "--dry-run=client", "-o", "yaml",
            ],
            check=True,
            capture_output=True,
        ).stdout
        subprocess.run(
            [args.client, "-n", args.namespace, "apply", "-f", "-"],
            input=secret,
            check=True,
        )
    print("ECR pull Secret diperbarui; ulangi sebelum token 12 jam kedaluwarsa.")


if __name__ == "__main__":
    main()
