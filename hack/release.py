# Copyright 2024 lke-operator contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import subprocess
import sys
import os
import requests
import yaml

import semver

import hack.utils as utils

logger = utils.setup(__name__)

def make(args: list[str]) -> None:
    rc = subprocess.run(
        ["make"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    if rc.returncode != 0:
        raise Exception(rc.stderr)

def git(args: list[str]) -> None:
    rc = subprocess.run(
        ["git"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    if rc.returncode != 0:
        raise Exception(rc.stderr)

def _parse_version(version: str) -> tuple[str, bool]:
    """
    Parses the given version string and returns a short version string and a boolean indicating whether it's a prerelease.

    Args:
        version (str): The version string to parse.

    Returns:
        Tuple[str, bool]: A tuple containing the short version string and a boolean indicating whether it's a prerelease.
    """
    short_version = ""
    is_prerelease = True

    if version == "main":
        short_version = version
    else:
        try:
            sv = semver.Version.parse(version.removeprefix("v"))
            short_version = f"v{sv.major}.{sv.minor}"
            is_prerelease = sv.prerelease is not None
        except ValueError as e:
            logger.warn(e)

    logger.info("%s (is_prerelease: %s)", short_version, is_prerelease)
    return short_version, is_prerelease


def _get_latest_kubernetes_release() -> str:
    url = "https://api.github.com/repos/kubernetes/kubernetes/releases/latest"
    response = requests.get(url)

    if response.status_code == 200:
        latest_release = response.json()['tag_name']
        return latest_release
    else:
        raise Exception(f"Failed to fetch the latest release. Status code: {response.status_code}")

def _replace_kubernetes_version(file_path: str, new_version: str) -> None:
    # Temporary file path
    temp_file_path = file_path + '.tmp'

    with open(file_path, 'r') as file, open(temp_file_path, 'w') as temp_file:
        for line in file:
            # Check if the line contains the kubernetesVersion key
            if 'kubernetesVersion' in line:
                # Split the line at the colon and replace the version
                key, _ = line.split(':')
                # Preserve the formatting by adding back the key and the new version
                temp_file.write(f"{key}: '{new_version}'\n")
            else:
                # Write the original line if it doesn't contain kubernetesVersion
                temp_file.write(line)

    # Replace the original file with the modified file
    os.replace(temp_file_path, file_path)

def _create_kustomization(resources, image_name, new_tag):
    kustomization = {
        "apiVersion": "kustomize.config.k8s.io/v1beta1",
        "kind": "Kustomization",
        "resources": resources,
        "images": [
            {
                "name": image_name,
                "newTag": new_tag
            }
        ]
    }
    return kustomization

def _write_kustomization(kustomization, filepath):
    with open(filepath, 'w') as file:
        yaml.dump(kustomization, file)

def run(args=sys.argv):
    """
    Runs the release script.

    Args:
        args: The arguments to the script.
    """
    parser = argparse.ArgumentParser(
        prog="publish",
        description="Script to build and publish documentation of lke-operator docs",
    )
    parser.add_argument(
        "--version",
        help="Tagged version which should be built",
        default="main",
        type=str,
        required=False,
    )
    parser.add_argument(
        "--config",
        help="Path to crd-ref-docs config",
        default="./docs/.crd-ref-docs.yaml",
        type=str,
        required=False,
    )
    parser.add_argument(
        "--image",
        help="Reference of the default image",
        default="ghcr.io/anza-labs/lke-operator",
        type=str,
        required=False,
    )

    args = parser.parse_args(args=args[1:])

    resources = [
        "./config/crd",
        "./config/manager",
        "./config/rbac"
    ]

    version, _ = _parse_version(version=args.version)
    kube_version, _ = _parse_version(version=_get_latest_kubernetes_release())
    _replace_kubernetes_version(args.config, kube_version)
    git(["checkout", "-b", f"release-{version}"])
    make(["manifests", "api-docs"])
    kustomization = _create_kustomization(resources=resources, image_name=args.image, new_tag=args.version)
    _write_kustomization(kustomization=kustomization, filepath="./kustomization.yaml")
