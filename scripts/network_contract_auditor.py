#!/usr/bin/env python3
from __future__ import annotations  # Lets Python use modern type hints safely

import argparse  # Handles command-line flags like --json
import json  # Prints machine-readable output
import sys  # Lets us exit with a non-zero code when errors exist
from dataclasses import asdict, dataclass  # Gives us clean report objects
from pathlib import Path  # Safer path handling than raw strings
from typing import Any  # Used for flexible YAML values

try:
    import yaml  # Parses docker-compose.yml
except ImportError:
    print("Missing dependency: PyYAML. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


PROJECT_ROOT = Path(__file__).resolve().parents[1]  # Pokemon/ directory
DEFAULT_COMPOSE_FILE = PROJECT_ROOT / "docker-compose.yml"  # Main compose file


@dataclass(frozen=True)
class Finding:
    service: str  # Compose service name, like server or api-consumer
    check: str  # What rule we checked
    status: str  # OK, WARNING, or ERROR
    message: str  # Human-readable result
    expected: str | None = None  # What should be true
    actual: str | None = None  # What was found


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Audit Pokemon Docker network contracts")
    parser.add_argument(
        "--compose-file",
        default=str(DEFAULT_COMPOSE_FILE),
        help="Path to docker-compose.yml",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print JSON instead of human-readable text",
    )
    return parser.parse_args()


def load_compose(path: Path) -> dict[str, Any]:
    if not path.exists():  # Make sure the compose file exists
        raise SystemExit(f"Compose file not found: {path}")

    with path.open("r", encoding="utf-8") as file:  # Read YAML as text
        data = yaml.safe_load(file)  # Convert YAML into Python dict/list values

    if not isinstance(data, dict):  # Compose should be a dictionary
        raise SystemExit(f"Compose file is not valid YAML: {path}")

    return data


def services(compose: dict[str, Any]) -> dict[str, Any]:
    value = compose.get("services", {})  # Get top-level services block
    return value if isinstance(value, dict) else {}


def get_service(compose: dict[str, Any], name: str) -> dict[str, Any]:
    service = services(compose).get(name, {})  # Find one service config
    return service if isinstance(service, dict) else {}


def normalize_environment(config: dict[str, Any]) -> dict[str, str]:
    env = config.get("environment", {})  # Compose supports dict or list env formats
    result: dict[str, str] = {}

    if isinstance(env, dict):  # Example: {"POSTGRES_HOST": "postgres"}
        for key, value in env.items():
            result[str(key)] = "" if value is None else str(value)
        return result

    if isinstance(env, list):  # Example: ["POKETCG_BASE_URL=http://poketcg:8765"]
        for item in env:
            if not isinstance(item, str):  # Ignore unusual values
                continue
            if "=" not in item:  # Handles env passthrough like "SOME_VAR"
                result[item] = ""
                continue
            key, _, value = item.partition("=")  # Split only on the first =
            result[key.strip()] = value.strip()
        return result

    return result


def normalize_depends_on(config: dict[str, Any]) -> list[str]:
    depends_on = config.get("depends_on", [])  # Compose supports list or dict depends_on

    if isinstance(depends_on, dict):  # Example: {"postgres": {"condition": "service_healthy"}}
        return list(depends_on.keys())

    if isinstance(depends_on, list):  # Example: ["server"]
        return [str(item) for item in depends_on]

    return []


def has_port(config: dict[str, Any], expected_container_port: str) -> bool:
    ports = config.get("ports", []) or []  # Published host ports
    expose = config.get("expose", []) or []  # Internal-only exposed ports

    combined = [str(item) for item in ports] + [str(item) for item in expose]

    for item in combined:
        if item.endswith(f":{expected_container_port}"):  # Example: 127.0.0.1:8765:8765
            return True
        if item == expected_container_port:  # Example: expose: ["8765"]
            return True

    return False


def add_service_exists(findings: list[Finding], compose: dict[str, Any], name: str) -> None:
    exists = name in services(compose)  # Check service presence
    findings.append(
        Finding(
            service=name,
            check="service exists",
            status="OK" if exists else "ERROR",
            message=f"{name} service is defined" if exists else f"{name} service is missing",
            expected="service present in docker-compose.yml",
            actual="present" if exists else "missing",
        )
    )


def add_env_check(
    findings: list[Finding],
    compose: dict[str, Any],
    service_name: str,
    env_name: str,
    expected_value: str,
    allow_missing_from_env_file: bool = False,
) -> None:
    config = get_service(compose, service_name)  # Get service config
    env = normalize_environment(config)  # Convert env list/dict to one dict
    actual = env.get(env_name)  # Read target env var

    if actual == expected_value:  # Exact expected value
        status = "OK"
        message = f"{env_name} is correct"
    elif actual is None and allow_missing_from_env_file and "env_file" in config:
        status = "WARNING"  # It may be in .env, so warn instead of hard error
        message = f"{env_name} is not inline; verify it in env_file"
    elif actual is None:
        status = "ERROR"
        message = f"{env_name} is missing"
    elif "localhost" in actual or "127.0.0.1" in actual:
        status = "ERROR"
        message = f"{env_name} uses localhost inside Docker"
    else:
        status = "ERROR"
        message = f"{env_name} does not match expected Docker service URL"

    findings.append(
        Finding(
            service=service_name,
            check=f"env {env_name}",
            status=status,
            message=message,
            expected=expected_value,
            actual=actual,
        )
    )


def add_env_contains_check(
    findings: list[Finding],
    compose: dict[str, Any],
    service_name: str,
    env_name: str,
    required_text: str,
    allow_missing_from_env_file: bool = True,
) -> None:
    config = get_service(compose, service_name)  # Get service config
    env = normalize_environment(config)  # Normalize environment block
    actual = env.get(env_name)  # Get actual env value

    if actual and required_text in actual:  # Good: contains service name/port
        status = "OK"
        message = f"{env_name} points at {required_text}"
    elif actual is None and allow_missing_from_env_file and "env_file" in config:
        status = "WARNING"
        message = f"{env_name} is not inline; verify it in env_file"
    elif actual is None:
        status = "ERROR"
        message = f"{env_name} is missing"
    elif "localhost" in actual or "127.0.0.1" in actual:
        status = "ERROR"
        message = f"{env_name} uses localhost inside Docker"
    else:
        status = "ERROR"
        message = f"{env_name} does not contain {required_text}"

    findings.append(
        Finding(
            service=service_name,
            check=f"env {env_name}",
            status=status,
            message=message,
            expected=f"contains {required_text}",
            actual=actual,
        )
    )


def add_depends_on_check(
    findings: list[Finding],
    compose: dict[str, Any],
    service_name: str,
    required_dependencies: list[str],
) -> None:
    config = get_service(compose, service_name)  # Get service config
    actual_dependencies = normalize_depends_on(config)  # Convert depends_on to list

    for dependency in required_dependencies:
        exists = dependency in actual_dependencies  # Check one required dependency
        findings.append(
            Finding(
                service=service_name,
                check=f"depends_on {dependency}",
                status="OK" if exists else "WARNING",
                message=(
                    f"{service_name} depends on {dependency}"
                    if exists
                    else f"{service_name} does not declare depends_on {dependency}"
                ),
                expected=dependency,
                actual=", ".join(actual_dependencies) if actual_dependencies else "none",
            )
        )


def add_port_check(
    findings: list[Finding],
    compose: dict[str, Any],
    service_name: str,
    expected_container_port: str,
) -> None:
    config = get_service(compose, service_name)  # Get service config
    ok = has_port(config, expected_container_port)  # Check ports/expose entries

    findings.append(
        Finding(
            service=service_name,
            check=f"port {expected_container_port}",
            status="OK" if ok else "WARNING",
            message=(
                f"{service_name} exposes container port {expected_container_port}"
                if ok
                else f"{service_name} does not expose container port {expected_container_port}"
            ),
            expected=expected_container_port,
            actual=str(config.get("ports") or config.get("expose") or "none"),
        )
    )


def build_report(compose: dict[str, Any], compose_file: Path) -> dict[str, Any]:
    findings: list[Finding] = []  # Store every check result

    required_services = [
        "postgres",
        "redis",
        "rabbitmq",
        "poketcg",
        "server",
        "api-consumer",
        "analytics-engine",
        "scraping-service",
        "client",
    ]

    for name in required_services:
        add_service_exists(findings, compose, name)  # Verify key services exist

    add_env_check(
        findings,
        compose,
        "server",
        "POKETCG_BASE_URL",
        "http://poketcg:8765",
    )

    add_env_check(
        findings,
        compose,
        "api-consumer",
        "GO_INTERNAL_URL",
        "http://server:3001/api/internal/watchlist-targets",
    )

    add_env_contains_check(
        findings,
        compose,
        "server",
        "POSTGRES_HOST",
        "postgres",
        allow_missing_from_env_file=True,
    )

    add_env_contains_check(
        findings,
        compose,
        "server",
        "REDIS_URL",
        "redis:6379",
        allow_missing_from_env_file=True,
    )

    add_env_contains_check(
        findings,
        compose,
        "server",
        "RABBITMQ_URL",
        "rabbitmq:5672",
        allow_missing_from_env_file=True,
    )

    add_env_contains_check(
        findings,
        compose,
        "api-consumer",
        "RABBITMQ_URL",
        "rabbitmq:5672",
        allow_missing_from_env_file=True,
    )

    add_depends_on_check(
        findings,
        compose,
        "server",
        ["postgres", "redis", "rabbitmq", "poketcg"],
    )

    add_depends_on_check(
        findings,
        compose,
        "api-consumer",
        ["postgres", "rabbitmq"],
    )

    add_depends_on_check(
        findings,
        compose,
        "client",
        ["server"],
    )

    add_port_check(findings, compose, "server", "3001")
    add_port_check(findings, compose, "client", "80")
    add_port_check(findings, compose, "poketcg", "8765")
    add_port_check(findings, compose, "postgres", "5432")
    add_port_check(findings, compose, "rabbitmq", "5672")
    add_port_check(findings, compose, "redis", "6379")

    ok_count = sum(1 for finding in findings if finding.status == "OK")
    warning_count = sum(1 for finding in findings if finding.status == "WARNING")
    error_count = sum(1 for finding in findings if finding.status == "ERROR")

    verdict = "valid" if error_count == 0 else "invalid"

    return {
        "compose_file": str(compose_file),
        "summary": {
            "ok": ok_count,
            "warnings": warning_count,
            "errors": error_count,
            "verdict": verdict,
        },
        "findings": [asdict(finding) for finding in findings],
    }


def print_human_report(report: dict[str, Any]) -> None:
    print("\nPokemon Network Contract Audit")
    print(f"Compose file: {report['compose_file']}\n")

    for finding in report["findings"]:
        print(f"{finding['service']} - {finding['check']}")
        print(f"  {finding['status']}: {finding['message']}")
        if finding["expected"] is not None:
            print(f"  expected: {finding['expected']}")
        if finding["actual"] is not None:
            print(f"  actual:   {finding['actual']}")
        print()

    summary = report["summary"]
    print("Summary")
    print(f"  OK:       {summary['ok']}")
    print(f"  WARNINGS: {summary['warnings']}")
    print(f"  ERRORS:   {summary['errors']}")
    print(f"  Verdict:  network contract is {summary['verdict']}")


def main() -> int:
    args = parse_args()  # Read CLI flags
    compose_file = Path(args.compose_file).resolve()  # Normalize compose path
    compose = load_compose(compose_file)  # Parse docker-compose.yml
    report = build_report(compose, compose_file)  # Run checks

    if args.json:
        print(json.dumps(report, indent=2))  # Machine-readable output
    else:
        print_human_report(report)  # Human-readable output

    return 1 if report["summary"]["errors"] > 0 else 0  # Fail only on errors


if __name__ == "__main__":
    raise SystemExit(main())
