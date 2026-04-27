import json
import pathlib
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]
BINARY = ROOT / ".build" / "release" / "apfel"


def test_benchmark_reports_real_speedups():
    result = subprocess.run(
        [str(BINARY), "--benchmark", "-o", "json"],
        text=True,
        capture_output=True,
        timeout=180,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    benchmarks = {entry["name"]: entry for entry in payload["benchmarks"]}

    for name in [
        "trim_newest_first",
        "trim_oldest_first",
        "message_text_content",
        "tool_schema_convert",
        "request_body_capture_disabled",
        "stream_debug_capture_disabled",
    ]:
        entry = benchmarks[name]
        assert entry["validated"] is True, entry
        assert entry["baseline_avg_ms"] is not None, entry
        assert entry["speedup_ratio"] is not None, entry
        assert entry["speedup_ratio"] > 1.0, entry

    for name in [
        "context_manager_make_session",
        "request_pipeline_noninference",
        "request_decode",
        "tool_call_detect",
        "response_encode",
    ]:
        assert benchmarks[name]["current_avg_ms"] >= 0
