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
import os
import subprocess
import sys

import requests
import semver
import yaml

import hack.utils as utils

logger = utils.setup(__name__)


def make(args: list[str]) -> None:
    """
    Runs the 'make' command with the provided arguments.

    Args:
        args (list[str]): List of arguments to pass to the 'make' command.
    """
    subprocess.run(
        ["make"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
        check=True,
    )


def git(args: list[str]) -> None:
    """
    Runs the 'git' command with the provided arguments.

    Args:
        args (list[str]): List of arguments to pass to the 'git' command.
    """
    subprocess.run(
        ["git"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
        check=True,
    )


def _branch_prep(version: str, full_version: str):
    """
    Prepares the branch for the release.

    If the branch exists, switches to it; otherwise, creates a new branch.
    Merges the main branch into the release branch and pushes the changes.

    Args:
        version (str): The short version string.
        full_version (str): The full version string.
    """
    branch = f"release-{version}"
    if _branch_exists(branch):
        _switch_to_branch(branch)
    else:
        _create_branch(branch)
    git(
        [
            "merge",
            "main",
            "-m",
            f"chore({version}): merge changes for {full_version}",
            "--signoff",
        ]
    )
    git(["push", "origin", branch])


def _branch_exists(branch_name: str) -> bool:
    """
    Checks if a Git branch exists.

    Args:
        branch_name (str): The name of the branch to check.

    Returns:
        bool: True if the branch exists, False otherwise.
    """
    result = subprocess.run(
        ["git", "branch", "--list", branch_name], capture_output=True, text=True
    )
    return branch_name in result.stdout


def _switch_to_branch(branch_name: str) -> None:
    """
    Switches to the specified Git branch.

    Args:
        branch_name (str): The name of the branch to switch to.
    """
    git(["checkout", branch_name])


def _create_branch(branch_name: str) -> None:
    """
    Creates and switches to a new Git branch.

    Args:
        branch_name (str): The name of the branch to create.
    """
    git(["checkout", "-b", branch_name])


def _parse_version(version: str) -> str:
    """
    Parses the given version string and returns a short version string.

    Args:
        version (str): The version string to parse.

    Returns:
        str: The short version string.
    """
    short_version = ""

    if version == "main":
        short_version = version
    else:
        try:
            sv = semver.Version.parse(version.removeprefix("v"))
            short_version = f"{sv.major}.{sv.minor}"
        except ValueError as e:
            logger.warn(e)

    logger.info("%s", short_version)
    return short_version


def _get_latest_kubernetes_release() -> str:
    """
    Fetches the latest Kubernetes release version from the GitHub API.

    Returns:
        str: The latest Kubernetes release version.

    Raises:
        Exception: If the API request fails.
    """
    url = "https://api.github.com/repos/kubernetes/kubernetes/releases/latest"
    response = requests.get(url)

    if response.status_code == 200:
        latest_release = response.json()["tag_name"]
        return latest_release
    else:
        raise Exception(
            f"Failed to fetch the latest release. Status code: {response.status_code}"
        )


def _replace_kubernetes_version(file_path: str, new_version: str) -> None:
    """
    Replaces the Kubernetes version in the specified configuration file.

    Args:
        file_path (str): The path to the configuration file.
        new_version (str): The new Kubernetes version to set.
    """
    temp_file_path = file_path + ".tmp"

    with open(file_path, "r") as file, open(temp_file_path, "w") as temp_file:
        for line in file:
            if "kubernetesVersion" in line:
                key, _ = line.split(":")
                temp_file.write(f"{key}: '{new_version}'\n")
            else:
                temp_file.write(line)

    os.replace(temp_file_path, file_path)


def _create_kustomization(resources, image_name, new_tag):
    """
    Creates a kustomization dictionary.

    Args:
        resources (list[str]): List of resource paths.
        image_name (str): Name of the image to replace.
        new_tag (str): New tag for the image.

    Returns:
        dict: Kustomization dictionary.
    """
    kustomization = {
        "resources": resources,
        "images": [{"name": image_name, "newTag": new_tag}],
    }
    return kustomization


def _release(version: str, full_version: str) -> None:
    """
    Creates a release by committing changes, pushing to the remote branch, and tagging the release.

    Args:
        version (str): The short version string.
        full_version (str): The full version string.
    """
    git(["add", "."])
    git(["commit", "-sm", f"chore({version}): create release commit {full_version}"])
    git(["push", "origin", f"release-{version}"])
    git(["tag", full_version])
    git(["push", "--tags"])


def _write_kustomization(kustomization, filepath) -> None:
    """
    Writes the kustomization dictionary to a file.

    Args:
        kustomization (dict): The kustomization dictionary.
        filepath (str): The path to the file where the kustomization will be written.
    """
    with open(filepath, "w") as file:
        yaml.dump(kustomization, file)


def run(args=sys.argv) -> None:
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

    resources = ["./config/crd", "./config/manager", "./config/rbac"]

    version = _parse_version(version=args.version)
    kube_version = _parse_version(version=_get_latest_kubernetes_release())
    _replace_kubernetes_version(args.config, kube_version)

    _branch_prep(version, args.version)

    kustomization = _create_kustomization(
        resources=resources, image_name=args.image, new_tag=args.version
    )
    _write_kustomization(kustomization=kustomization, filepath="./kustomization.yaml")
    make(["manifests", "api-docs"])

    _release(version, args.version)
