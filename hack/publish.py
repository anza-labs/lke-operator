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

import semver

import hack.utils as utils

logger = utils.setup(__name__)


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


def mike(args: list[str]) -> None:
    """
    Runs Mike with the given arguments.

    Args:
        args (list[str]): The list of arguments to pass to Mike.

    Raises:
        Exception: If Mike command execution returns a non-zero exit code.
    """
    subprocess.run(
        ["mike"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
        check=True,
    )


def is_initial() -> bool:
    """
    Checks if the current repository state is initial.

    Returns:
        bool: True if the repository state is initial, False otherwise.
    """
    logger.info("fetching gh-pages from origin")
    subprocess.run(
        ["git", "remote", "update"],
        stdout=sys.stdout,
        stderr=sys.stderr,
    )

    logger.info("getting ref of gh-pages")
    rc = subprocess.run(
        ["git", "show-ref", "refs/remotes/origin/gh-pages"],
        stdout=sys.stdout,
        stderr=sys.stderr,
    )

    logger.info(f"returncode={rc.returncode}")
    return rc.returncode != 0


def run(args=sys.argv):
    """
    Runs the publish script.

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

    args = parser.parse_args(args=args[1:])

    version, prerelease = _parse_version(version=args.version)
    if is_initial():
        logger.info("assuming initial release")

        logger.info("deploing latest")
        mike(["deploy", "--push", "--update-aliases", version, "latest"])

        logger.info("seting default as latest")
        mike(["set-default", "--push", "latest"])
    else:
        logger.info(
            f"not an initial release, but prerelease={prerelease} version={version}"
        )

        if prerelease:
            logger.info(f"deploying prerelease of {version}")
            mike(["deploy", "--push", version])
        else:
            logger.info(f"deploying release of {version}")
            mike(["deploy", "--push", "--update-aliases", version, "latest"])
