#!/usr/bin/env python3
"""Re-runs hermes_call_analytics against zvonari-golden-set.json and reports
match/mismatch per call. Read-only against the DB; calls the live
hermes_call_analytics container by default (no code changes needed to test
a deployed prompt), or import call_analytics_server directly with
--hermes-path to test an unreleased prompt change before deploying it.

Usage:
    python3 scripts/zvonari_golden_check.py
    python3 scripts/zvonari_golden_check.py --hermes-path ~/dev/hermes/services

Requires: docker exec access to invoices_postgres, and either
CALL_ANALYTICS_URL+CALL_ANALYTICS_TOKEN env vars (HTTP mode) or
OPENROUTER_API_KEY (direct-import mode, reads it from hermes/.env if unset).
"""
import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

GOLDEN_SET_PATH = Path(__file__).resolve().parent.parent / "zvonari-golden-set.json"


def fetch_call(call_id: str) -> dict:
    sql = (
        f"SELECT transcript_text, duration_sec, talk_time_sec, direction "
        f"FROM calls WHERE id='{call_id}';"
    )
    out = subprocess.run(
        ["docker", "exec", "invoices_postgres", "psql", "-U", "invoices_user", "-d", "invoices_db",
         "-t", "-A", "-F", "\x1f", "-c", sql],
        capture_output=True, text=True, check=True,
    )
    transcript, duration, talk_time, direction = out.stdout.rstrip("\n").split("\x1f", 3)
    return {
        "transcript": transcript,
        "duration_sec": int(duration),
        "talk_time_sec": int(talk_time),
        "direction": direction,
    }


_CAS = None


def analyze_via_module(hermes_path: str, call: dict, model_override: str = None) -> dict:
    global _CAS
    if _CAS is None:
        sys.path.insert(0, os.path.expanduser(hermes_path))
        import call_analytics_server as cas  # noqa: E402
        if not cas.OPENROUTER_API_KEY:
            env_path = os.path.expanduser(os.path.join(hermes_path, "..", ".env"))
            if os.path.exists(env_path):
                with open(env_path) as f:
                    for line in f:
                        if line.strip().startswith("OPENROUTER_API_KEY="):
                            cas.OPENROUTER_API_KEY = line.strip().split("=", 1)[1]
        _CAS = cas
    if model_override:
        _CAS.OPENROUTER_MODEL = model_override
    return _CAS.analyze_call(
        call["transcript"], call["duration_sec"], call["talk_time_sec"], call["direction"],
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--hermes-path", default=None,
                         help="Import call_analytics_server directly from this dir instead of hitting the live HTTP service")
    parser.add_argument("--model", default=None,
                         help="Override OPENROUTER_MODEL for this run (direct-import mode only)")
    args = parser.parse_args()

    golden = json.loads(GOLDEN_SET_PATH.read_text())
    entries = golden["entries"]

    total_checked = 0
    matches = 0
    mismatches = []
    skipped = []

    for entry in entries:
        call_id = entry["call_id"]
        expected = entry.get("expected_outcome")
        if expected is None:
            skipped.append(call_id)
            continue
        call = fetch_call(call_id)
        try:
            if args.hermes_path:
                analytics = analyze_via_module(args.hermes_path, call, args.model)
            else:
                raise NotImplementedError("HTTP mode not wired yet — pass --hermes-path")
        except Exception as exc:  # noqa: BLE001
            mismatches.append((call_id, expected, f"ERROR: {exc}", entry.get("confidence")))
            continue

        total_checked += 1
        actual = analytics.get("outcome")
        if actual == expected:
            matches += 1
        else:
            mismatches.append((call_id, expected, actual, entry.get("confidence")))

    print(f"Golden set: {len(entries)} entries, {len(skipped)} skipped (no fixed expectation), {total_checked} checked")
    print(f"Matches: {matches}/{total_checked}")
    if mismatches:
        print("\nMismatches:")
        for call_id, expected, actual, confidence in mismatches:
            print(f"  [{confidence}] {call_id}\n      expected: {expected}\n      actual:   {actual}")


if __name__ == "__main__":
    main()
