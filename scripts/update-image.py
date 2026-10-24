#!/usr/bin/env python3
"""Update a Kubernetes or OpenShift Kustomize image after a successful push."""

import argparse
from pathlib import Path
import re

parser = argparse.ArgumentParser()
parser.add_argument("--target", required=True, choices=("eks", "ec2", "openshift"))
parser.add_argument("--repository-url", required=True)
parser.add_argument("--tag", required=True)
args = parser.parse_args()

if not re.fullmatch(r"[0-9a-f]{40}", args.tag):
    parser.error("tag must be a full Git commit SHA")
ecr_pattern = r"[0-9]+\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com/[a-z0-9/_-]+"
if not re.fullmatch(ecr_pattern, args.repository_url):
    parser.error(f"repository-url is invalid for {args.target}")

overlay = {"eks": "prod", "ec2": "ec2", "openshift": "openshift"}[args.target]
path = Path(f"k8s/overlays/{overlay}/kustomization.yaml")
data = path.read_text()
for key, value in (("newName", args.repository_url), ("newTag", args.tag)):
    data, count = re.subn(rf"(?m)^    {key}: .+$", f"    {key}: {value}", data)
    if count != 1:
        raise SystemExit(f"expected exactly one {key} entry, found {count}")
path.write_text(data)
