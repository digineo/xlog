#!/usr/bin/env python3
"""Release all modules of this repository with a shared version.

Usage:
    scripts/release.py prepare X.Y.Z   open a release pull request
    scripts/release.py tag X.Y.Z       tag the merged release on main
    scripts/release.py all X.Y.Z       prepare, wait for the merge, then tag
    scripts/release.py TEST            run the self-tests

"prepare" moves the unreleased CHANGELOG entries into a section for the
new version, makes the sub-modules require the new xlog version, and
opens a pull request. Once it is merged, "tag" tags main as vX.Y.Z and
each sub-module as <dir>/vX.Y.Z, and pushes all tags at once.
"""

import datetime
import re
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CHANGELOG = ROOT / "CHANGELOG.md"
MODULE = "github.com/digineo/xlog"


def run(*cmd, cwd=ROOT):
    try:
        return subprocess.run(
            cmd, cwd=cwd, check=True, text=True, stdout=subprocess.PIPE
        ).stdout.strip()
    except subprocess.CalledProcessError:
        sys.exit(f"release: {' '.join(cmd)} failed")


def submodules():
    return sorted(p.parent for p in ROOT.glob("*/go.mod"))


def parse_version(v):
    return tuple(int(n) for n in v.split("."))


def release_changelog(text, version, date):
    """Moves the unreleased entries into a section for version."""
    head = "## [Unreleased]\n"
    if head not in text:
        raise ValueError("missing Unreleased section")
    if not text.split(head, 1)[1].split("\n## [", 1)[0].strip():
        raise ValueError("no unreleased changes")
    link = re.search(
        r"^\[Unreleased\]: (\S+)/compare/(\S+)\.\.\.HEAD$", text, re.MULTILINE
    )
    if not link:
        raise ValueError("missing Unreleased link")

    base, prev = link.groups()
    text = text.replace(
        link.group(0),
        "\n".join(
            [
                f"[Unreleased]: {base}/compare/v{version}...HEAD",
                f"[{version}]: {base}/compare/{prev}...v{version}",
            ]
        ),
    )
    return text.replace(head, f"{head}\n## [{version}] - {date}\n", 1)


def release_notes(text, version):
    """Extracts the CHANGELOG section of version."""
    section = text.split(f"\n## [{version}] - ", 1)[1].split("\n", 1)[1]
    return re.split(r"\n## \[|\n\[Unreleased\]: ", section, maxsplit=1)[0].strip()


def sync_main():
    if run("git", "branch", "--show-current") != "main":
        sys.exit("release: not on main")
    if run("git", "status", "--porcelain"):
        sys.exit("release: working tree not clean")
    run("git", "fetch", "--tags", "origin")
    run("git", "merge", "--ff-only", "origin/main")
    if run("git", "rev-parse", "HEAD") != run("git", "rev-parse", "origin/main"):
        sys.exit("release: main has unpushed commits")


def prepare(version):
    sync_main()
    tags = run("git", "tag", "--list", "v*").split()
    latest = max(
        (parse_version(t[1:]) for t in tags if re.fullmatch(r"v\d+\.\d+\.\d+", t)),
        default=(0, 0, 0),
    )
    if parse_version(version) <= latest:
        sys.exit(f"release: {version} is not newer than {'.'.join(map(str, latest))}")
    try:
        text = release_changelog(
            CHANGELOG.read_text(),
            version,
            datetime.datetime.now(datetime.UTC).date().isoformat(),
        )
    except ValueError as e:
        sys.exit(f"release: CHANGELOG.md: {e}")

    branch = f"release/v{version}"
    run("git", "switch", "--create", branch)
    CHANGELOG.write_text(text)
    for mod in submodules():
        run("go", "mod", "edit", f"-require={MODULE}@v{version}", cwd=mod)
    run("git", "commit", "--all", "--message", f"chore: release v{version}")
    run("git", "push", "--set-upstream", "origin", branch)
    pr = run(
        *("gh", "pr", "create"),
        *("--base", "main"),
        *("--head", branch),
        *("--title", f"chore: release v{version}"),
        *("--body", release_notes(text, version)),
    )
    print(pr)
    print(f"Once merged, run: scripts/release.py tag {version}")
    return pr


def tag(version):
    sync_main()
    if f"\n## [{version}] - " not in CHANGELOG.read_text():
        sys.exit(
            f"release: CHANGELOG.md has no section for {version}, is the release PR merged?"
        )

    tags = [f"v{version}"]
    for mod in submodules():
        if f"{MODULE} v{version}\n" not in (mod / "go.mod").read_text():
            sys.exit(f"release: {mod.name}/go.mod does not require {MODULE} v{version}")
        tags.append(f"{mod.name}/v{version}")

    for t in tags:
        run("git", "tag", t)
    run("git", "push", "--atomic", "origin", *tags)
    print("Pushed tags:", *tags)


def wait_merged(pr):
    print(f"Waiting for {pr} to be merged, press Ctrl-C to abort.")
    try:
        while (
            state := run("gh", "pr", "view", pr, "--json=state", "--jq=.state")
        ) == "OPEN":
            time.sleep(15)
    except KeyboardInterrupt:
        sys.exit("release: aborted")
    if state != "MERGED":
        sys.exit(f"release: {pr} is {state.lower()}")


def release_all(version):
    wait_merged(prepare(version))
    run("git", "switch", "main")
    tag(version)
    # rebase merges rewrite the commit, so git can't tell the branch is merged
    run("git", "branch", "--delete", "--force", f"release/v{version}")


COMMANDS = {"prepare": prepare, "tag": tag, "all": release_all}


def main(argv):
    if (
        len(argv) != 2
        or argv[0] not in COMMANDS
        or not re.fullmatch(r"\d+\.\d+\.\d+", argv[1])
    ):
        sys.exit(__doc__ or "usage: scripts/release.py prepare|tag|all X.Y.Z")
    COMMANDS[argv[0]](argv[1])


def test():
    changelog = """# Changelog

## [Unreleased]

### Added

- New feature.

## [1.1.0] - 2026-08-11

- Old feature.

[Unreleased]: https://github.com/digineo/xlog/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/digineo/xlog/compare/v1.0.0...v1.1.0
"""
    released = """# Changelog

## [Unreleased]

## [1.2.0] - 2026-10-14

### Added

- New feature.

## [1.1.0] - 2026-08-11

- Old feature.

[Unreleased]: https://github.com/digineo/xlog/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/digineo/xlog/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/digineo/xlog/compare/v1.0.0...v1.1.0
"""
    for name, text, expected in [
        ("release", changelog, released),
        (
            "no changes",
            changelog.replace("### Added\n\n- New feature.\n\n", ""),
            "no unreleased changes",
        ),
        (
            "no section",
            changelog.replace("## [Unreleased]\n", ""),
            "missing Unreleased section",
        ),
        (
            "no link",
            changelog.replace("[Unreleased]:", "[unreleased]:"),
            "missing Unreleased link",
        ),
    ]:
        try:
            got = release_changelog(text, "1.2.0", "2026-10-14")
        except ValueError as e:
            got = str(e)
        assert got == expected, f"{name}: {got!r}"

    assert release_notes(released, "1.2.0") == "### Added\n\n- New feature."
    assert release_notes(released, "1.1.0") == "- Old feature."
    print("ok")


if __name__ == "__main__":
    if sys.argv[1:] == ["TEST"]:
        test()
    else:
        main(sys.argv[1:])
