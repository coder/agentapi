#!/usr/bin/env python3
"""Quick test script for the Python package."""

import subprocess
import sys
from pathlib import Path


def run_command(cmd, description):
    """Run a command and report results."""
    print(f"\n{'='*60}")
    print(f"Testing: {description}")
    print(f"Command: {' '.join(cmd)}")
    print('='*60)

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=30
        )

        if result.stdout:
            print("STDOUT:", result.stdout)
        if result.stderr:
            print("STDERR:", result.stderr)

        if result.returncode == 0:
            print(f"✓ {description} - PASSED")
            return True
        else:
            print(f"✗ {description} - FAILED (exit code: {result.returncode})")
            return False

    except subprocess.TimeoutExpired:
        print(f"✗ {description} - TIMEOUT")
        return False
    except Exception as e:
        print(f"✗ {description} - ERROR: {e}")
        return False


def main():
    """Run package tests."""
    print("Starting agentapi Python package tests...")

    tests = [
        (
            [sys.executable, "-m", "pip", "install", "-e", "."],
            "Install package in development mode"
        ),
        (
            [sys.executable, "-c", "import agentapi; print(f'Version: {agentapi.__version__}')"],
            "Import package and check version"
        ),
        (
            [sys.executable, "-c", "from agentapi import ensure_binary; print('Import successful')"],
            "Import ensure_binary function"
        ),
        (
            ["agentapi", "--version"],
            "Run agentapi --version"
        ),
        (
            ["agentapi", "--help"],
            "Run agentapi --help"
        ),
        (
            [sys.executable, "-m", "agentapi", "--version"],
            "Run python -m agentapi --version"
        ),
    ]

    results = []
    for cmd, description in tests:
        results.append(run_command(cmd, description))

    # Summary
    print("\n" + "="*60)
    print("TEST SUMMARY")
    print("="*60)
    passed = sum(results)
    total = len(results)
    print(f"Passed: {passed}/{total}")

    if passed == total:
        print("\n✓ All tests passed!")
        return 0
    else:
        print(f"\n✗ {total - passed} test(s) failed")
        return 1


if __name__ == "__main__":
    sys.exit(main())
