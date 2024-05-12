import argparse
import subprocess
import sys

import semver

import scripts.utils as utils

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
    rc = subprocess.run(
        ["mike"] + args,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    if rc.returncode != 0:
        raise Exception(rc.stderr)


def is_initial() -> bool:
    """
    Checks if the current repository state is initial.

    Returns:
        bool: True if the repository state is initial, False otherwise.
    """
    rc = subprocess.run(
        ["git", "show-ref", "--quiet", "refs/heads/gh-pages"],
    )
    return rc.returncode != 0


def run(args=sys.argv):
    """
    Runs the publish script.

    Args:
        args: The arguments to the script.
    """
    parser = argparse.ArgumentParser(
        prog="publish",
        description="Script to build and publish documentation of registry-operator docs",
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
        mike(["deploy", "--push", "--update-aliases", version, "latest"])
        mike(["set-default", "--push", "latest"])
    else:
        if prerelease:
            mike(["deploy", "--push", "--update-aliases", version])
        else:
            mike(["deploy", "--push", "--update-aliases", version, "latest"])
