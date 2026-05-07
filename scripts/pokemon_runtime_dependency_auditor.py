#!/usr/bin/env python3  # Tells Linux/Ubuntu to run this file using python3 when executed directly.
"""
pokemon_runtime_dependency_auditor.py

This script audits the Pokemon Docker stack.

It compares:
1. What docker-compose.yml says should exist.
2. What Docker and the host machine say is actually running.

This is infrastructure code because it checks services, ports, dependencies,
health, risk flags, and failure impact.
"""

from __future__ import annotations  # Lets Python handle newer type hints more cleanly.

import json  # Used to read/write JSON data.
import re  # Used for regex pattern matching, especially env var port parsing.
import socket  # Used to test real TCP connections to localhost ports.
import subprocess  # Used to run system commands like docker and ss.
import sys  # Used for exiting the script and printing errors to stderr.
from dataclasses import asdict, dataclass, field  # Used to build clean structured data objects.
from pathlib import Path  # Used for filesystem paths in a cleaner way than raw strings.
from typing import Any  # Used when a value can be any type.

try:  # Try importing PyYAML.
    import yaml  # Used to parse docker-compose.yml.
except ImportError:  # Runs if PyYAML is not installed.
    print("PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)  # Show install help.
    sys.exit(1)  # Exit with error code 1 because the script cannot continue.


PROJECT_ROOT = Path("/home/iscjmz/shopify/shopify/Pokemon")  # Absolute path to the Pokemon project.
COMPOSE_FILE = PROJECT_ROOT / "docker-compose.yml"  # Full path to the Docker Compose file.
OUTPUT_FILE = Path("/tmp/pokemon_runtime_audit.json")  # Where the final JSON audit report will be written.


@dataclass  # Automatically creates __init__, repr, and other helper methods for this class.
class ServiceReport:  # Represents one service in the final audit report.
    service: str  # The compose service name, like postgres, redis, server, client.
    container_name: str | None  # The Docker container name, or None if not known.
    image: str | None  # The Docker image name, or None if the service is built locally.
    defined: bool = True  # Whether this service is defined in docker-compose.yml.
    running: bool = False  # Whether Docker says this service is currently running.
    status: str = "unknown"  # Runtime status, like running, exited, missing, unknown.
    healthy: bool | None = None  # True if healthy, False if unhealthy, None if no healthcheck.
    restart: str | None = None  # Restart policy from compose, like unless-stopped.
    depends_on: list[str] = field(default_factory=list)  # Services this service depends on.
    host_ports: list[str] = field(default_factory=list)  # Ports exposed on the host machine.
    internal_ports: list[str] = field(default_factory=list)  # Ports inside the container.
    volumes: list[str] = field(default_factory=list)  # Volumes mounted into the container.
    has_healthcheck: bool = False  # Whether compose defines a healthcheck for this service.
    risk_flags: list[str] = field(default_factory=list)  # Problems or warnings found by the audit.
    blast_radius: list[str] = field(default_factory=list)  # What breaks if this service fails.


def run_command(cmd: list[str]) -> subprocess.CompletedProcess:  # Runs a terminal command safely.
    return subprocess.run(cmd, capture_output=True, text=True)  # Capture stdout/stderr as strings.


def load_compose(path: Path) -> dict[str, Any]:  # Loads docker-compose.yml into a Python dictionary.
    if not path.exists():  # Check whether the compose file actually exists.
        raise FileNotFoundError(f"Compose file not found: {path}")  # Stop if the file is missing.

    with path.open("r", encoding="utf-8") as f:  # Open the YAML file in read mode.
        data = yaml.safe_load(f)  # Parse YAML safely into Python dict/list data.

    return data or {}  # Return parsed data, or empty dict if file is empty.


def clean_port_value(value: str) -> str:  # Cleans a port value from compose.
    value = value.strip().strip('"').strip("'")  # Remove spaces and quotes around the value.
    env_match = re.search(r":-(.*?)}", value)  # Find default value inside patterns like ${PORT:-3001}.
    return env_match.group(1) if env_match else value  # Return 3001 if matched, otherwise original value.


def parse_port_mapping(port_value: Any) -> tuple[list[str], list[str]]:  # Splits compose ports into host/internal ports.
    host_ports: list[str] = []  # Stores ports exposed on your computer.
    internal_ports: list[str] = []  # Stores ports used inside containers.

    if isinstance(port_value, int):  # Handles compose ports written as numbers, like 80.
        internal_ports.append(str(port_value))  # Convert number to string and save as internal port.
        return host_ports, internal_ports  # Return because no host port was defined.

    if not isinstance(port_value, str):  # Ignore weird formats this script does not support.
        return host_ports, internal_ports  # Return empty lists safely.

    parts = port_value.split(":")  # Split strings like 127.0.0.1:5432:5432 into pieces.

    if len(parts) >= 2:  # If there is at least host:container style mapping.
        internal_ports.append(clean_port_value(parts[-1]))  # Last piece is container/internal port.
        host_ports.append(clean_port_value(parts[-2]))  # Second-to-last piece is host port.
    else:  # Handles simple strings like "80".
        internal_ports.append(clean_port_value(parts[0]))  # No host port, only internal port.

    return host_ports, internal_ports  # Return both lists.


def normalize_depends_on(depends_on: Any) -> list[str]:  # Converts depends_on into a clean list of service names.
    if not depends_on:  # If depends_on is missing, None, or empty.
        return []  # No dependencies.

    if isinstance(depends_on, dict):  # Compose can use dict style with health conditions.
        return list(depends_on.keys())  # The dict keys are the dependency service names.

    if isinstance(depends_on, list):  # Compose can also use list style.
        return [str(item) for item in depends_on]  # Convert every dependency name to string.

    return []  # Unknown format, safely return no dependencies.


def parse_compose_services(compose: dict[str, Any]) -> dict[str, ServiceReport]:  # Converts compose services into reports.
    services = compose.get("services", {})  # Get the services section from docker-compose.yml.
    reports: dict[str, ServiceReport] = {}  # Create empty dict to store reports by service name.

    for service_name, config in services.items():  # Loop through every compose service.
        ports = config.get("ports", []) or []  # Get port mappings, or empty list.
        volumes = config.get("volumes", []) or []  # Get volume mappings, or empty list.
        depends_on = normalize_depends_on(config.get("depends_on"))  # Normalize dependencies.
        image = config.get("image")  # Get Docker image if service uses an image.
        container_name = config.get("container_name")  # Get explicit container name if set.
        restart = config.get("restart")  # Get restart policy.
        has_healthcheck = "healthcheck" in config  # True if service has a healthcheck block.

        host_ports: list[str] = []  # Start empty list for host ports.
        internal_ports: list[str] = []  # Start empty list for container ports.

        for port in ports:  # Loop through every port mapping.
            parsed_host_ports, parsed_internal_ports = parse_port_mapping(port)  # Parse one mapping.
            host_ports.extend(parsed_host_ports)  # Add parsed host ports to full list.
            internal_ports.extend(parsed_internal_ports)  # Add parsed internal ports to full list.

        reports[service_name] = ServiceReport(  # Create ServiceReport for this service.
            service=service_name,  # Store service name.
            container_name=container_name,  # Store container name.
            image=image,  # Store image name.
            restart=restart,  # Store restart policy.
            depends_on=depends_on,  # Store dependency list.
            volumes=[str(volume) for volume in volumes],  # Store volumes as strings.
            host_ports=host_ports,  # Store exposed host ports.
            internal_ports=internal_ports,  # Store internal container ports.
            has_healthcheck=has_healthcheck,  # Store whether healthcheck exists.
        )

    return reports  # Return all service reports.


def docker_ps() -> list[dict[str, Any]]:  # Gets Docker container list as Python dictionaries.
    result = run_command(["docker", "ps", "-a", "--format", "{{json .}}"])  # Run docker ps for all containers as JSON lines.

    if result.returncode != 0:  # If docker command failed.
        return []  # Return empty list so script does not crash.

    rows: list[dict[str, Any]] = []  # Store parsed container rows here.

    for line in result.stdout.splitlines():  # Loop through each JSON line from Docker.
        line = line.strip()  # Remove whitespace.

        if not line:  # Skip blank lines.
            continue  # Go to next line.

        try:  # Try parsing JSON.
            rows.append(json.loads(line))  # Convert JSON string into Python dict.
        except json.JSONDecodeError:  # If one line is bad JSON.
            continue  # Skip bad line.

    return rows  # Return all Docker container rows.


def docker_inspect(name: str) -> dict[str, Any] | None:  # Gets deep Docker info for one container.
    result = run_command(["docker", "inspect", name])  # Run docker inspect on the container.

    if result.returncode != 0:  # If inspect failed.
        return None  # Return None because no data exists.

    try:  # Try parsing inspect output.
        payload = json.loads(result.stdout)  # Docker inspect returns a JSON list.
    except json.JSONDecodeError:  # If JSON parsing fails.
        return None  # Return None safely.

    if not payload:  # If Docker returned an empty list.
        return None  # Return None.

    return payload[0]  # Return first container object from inspect result.


def get_listening_ports() -> set[int]:  # Reads host listening ports from ss command.
    result = run_command(["ss", "-tulpn"])  # Run ss to list TCP/UDP listening ports and processes.

    if result.returncode != 0:  # If ss failed.
        return set()  # Return empty set.

    ports: set[int] = set()  # Use a set so duplicate ports appear only once.

    for line in result.stdout.splitlines():  # Loop through ss output line by line.
        line = line.strip()  # Remove whitespace.

        if not line or line.startswith(("Netid", "State", "Rec-Q")):  # Skip headers and blank lines.
            continue  # Move to next line.

        parts = line.split()  # Split columns by whitespace.

        if len(parts) < 5:  # Need enough columns to read local address.
            continue  # Skip malformed line.

        local_address = parts[4]  # Local address column usually contains IP:PORT.

        if ":" not in local_address:  # If no colon, no port to parse.
            continue  # Skip it.

        port_str = local_address.rsplit(":", 1)[-1]  # Split from the right and grab port part.

        if port_str.isdigit():  # Make sure the port is numeric.
            ports.add(int(port_str))  # Convert to int and add to set.

    return ports  # Return all detected listening ports.


def check_local_tcp_port(port: int, timeout: float = 1.0) -> bool:  # Tries connecting to localhost:port.
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)  # Create IPv4 TCP socket.
    sock.settimeout(timeout)  # Prevent the connection attempt from hanging too long.

    try:  # Try connection test.
        return sock.connect_ex(("127.0.0.1", port)) == 0  # True if TCP connection succeeds.
    finally:  # Always run cleanup.
        sock.close()  # Close socket so we do not leak resources.


def find_container_for_service(  # Matches a compose service to a Docker container.
    service_name: str,  # Compose service name.
    report: ServiceReport,  # Existing report for that service.
    containers: list[dict[str, Any]],  # Docker ps container rows.
) -> dict[str, Any] | None:  # Returns container dict or None.
    expected_names = {  # Possible names this container might have.
        service_name,  # Example: postgres.
        report.container_name or "",  # Example: pokemontool_postgres.
        f"pokemontool_{service_name}",  # Example: pokemontool_redis.
        f"pokemon_{service_name}",  # Backup naming guess.
    }

    for container in containers:  # Loop through Docker containers.
        names = {  # Build names found in Docker metadata.
            str(container.get("Names", "")),  # Docker container name.
            str(container.get("Label", "")),  # Docker label if present.
        }

        if names & expected_names:  # Set intersection: do any names match?
            return container  # Return matching container.

        container_name = str(container.get("Names", ""))  # Get container name as string.

        if report.container_name and container_name == report.container_name:  # Exact match check.
            return container  # Return matching container.

    return None  # No container matched this service.


def read_health_from_inspect(inspect_data: dict[str, Any]) -> bool | None:  # Reads Docker healthcheck status.
    state = inspect_data.get("State", {})  # Get State section from docker inspect.
    health = state.get("Health")  # Get Health section if it exists.

    if not health:  # If no healthcheck exists.
        return None  # None means health is unknown/not configured.

    return health.get("Status") == "healthy"  # True only if Docker says healthy.


def enrich_with_runtime_state(  # Adds live Docker runtime data to reports.
    reports: dict[str, ServiceReport],  # Service reports from compose.
    containers: list[dict[str, Any]],  # Docker ps rows.
) -> None:  # Mutates reports in place, returns nothing.
    for service_name, report in reports.items():  # Loop through every service report.
        container = find_container_for_service(service_name, report, containers)  # Find matching Docker container.

        if not container:  # If no Docker container exists.
            report.running = False  # Mark as not running.
            report.status = "missing"  # Mark status as missing.
            continue  # Move to next service.

        container_name = str(container.get("Names", ""))  # Read container name from docker ps.
        report.container_name = report.container_name or container_name  # Fill container_name if missing.
        report.status = str(container.get("Status", "unknown"))  # Store docker ps status text.

        inspect_data = docker_inspect(container_name)  # Get deeper runtime details.

        if not inspect_data:  # If inspect failed.
            report.running = "Up" in report.status  # Fallback: infer running from docker ps status.
            continue  # Move to next service.

        state = inspect_data.get("State", {})  # Get Docker State object.
        report.running = bool(state.get("Running", False))  # Store real running boolean.
        report.status = str(state.get("Status", report.status))  # Store status like running/exited.
        report.healthy = read_health_from_inspect(inspect_data)  # Store healthcheck result.


def detect_risks(  # Adds warning messages to each service.
    reports: dict[str, ServiceReport],  # All service reports.
    listening_ports: set[int],  # Host ports currently listening.
) -> None:  # Mutates reports in place.
    for report in reports.values():  # Loop through each service report.
        if not report.running:  # If service is not running.
            report.risk_flags.append("container is not running")  # Add risk warning.

        if report.healthy is False:  # If healthcheck exists and is failing.
            report.risk_flags.append("container healthcheck is failing")  # Add risk warning.

        if report.restart in (None, "", "no"):  # If no useful restart policy exists.
            report.risk_flags.append("no restart policy configured")  # Add reliability warning.

        if report.host_ports and not report.has_healthcheck:  # If service exposes a port but no healthcheck.
            report.risk_flags.append("exposes host port but has no healthcheck")  # Add observability warning.

        for host_port in report.host_ports:  # Check every declared host port.
            if not host_port.isdigit():  # Ignore non-numeric port values.
                continue  # Move to next port.

            port = int(host_port)  # Convert port string to integer.

            if port not in listening_ports:  # If compose declares port but host is not listening.
                report.risk_flags.append(f"host port {port} is declared but not listening")  # Add warning.

            if not check_local_tcp_port(port):  # Try real TCP connection to localhost port.
                report.risk_flags.append(f"localhost TCP check failed for port {port}")  # Add warning.

        if report.service in {"postgres", "redis", "rabbitmq"} and report.host_ports:  # Infra dependencies exposed to host.
            report.risk_flags.append("infrastructure dependency exposes a host port")  # Add security awareness warning.

        for dependency in report.depends_on:  # Loop through dependencies for this service.
            dependency_report = reports.get(dependency)  # Find report for dependency.

            if dependency_report and not dependency_report.running:  # If dependency exists but is down.
                report.risk_flags.append(f"dependency is not running: {dependency}")  # Add dependency warning.


def calculate_blast_radius(reports: dict[str, ServiceReport]) -> None:  # Figures out what breaks if each service fails.
    reverse_dependencies: dict[str, list[str]] = {}  # Maps dependency to services that depend on it.

    for service_name, report in reports.items():  # Loop through every service.
        for dependency in report.depends_on:  # Loop through what that service depends on.
            reverse_dependencies.setdefault(dependency, []).append(service_name)  # Add reverse dependency relationship.

    for service_name, report in reports.items():  # Loop through every service again.
        affected_services = reverse_dependencies.get(service_name, [])  # Get services that depend on this one.

        for affected in affected_services:  # Loop through affected services.
            report.blast_radius.append(f"{affected} depends on {service_name}")  # Add dependency impact note.

        if service_name == "postgres":  # Special meaning for Postgres.
            report.blast_radius.append("persistent app data becomes unavailable")  # Explain impact.

        if service_name == "redis":  # Special meaning for Redis.
            report.blast_radius.append("cache-backed features may slow down or degrade")  # Explain impact.

        if service_name == "rabbitmq":  # Special meaning for RabbitMQ.
            report.blast_radius.append("async jobs and event flow may stop")  # Explain impact.

        if service_name == "server":  # Special meaning for backend server.
            report.blast_radius.append("frontend and API clients lose backend access")  # Explain impact.

        if service_name == "client":  # Special meaning for frontend client.
            report.blast_radius.append("users lose browser access to the app UI")  # Explain impact.

        if service_name == "loki":  # Special meaning for Loki.
            report.blast_radius.append("centralized log storage becomes unavailable")  # Explain impact.

        if service_name == "promtail":  # Special meaning for Promtail.
            report.blast_radius.append("container logs stop shipping to Loki")  # Explain impact.

        if service_name == "grafana":  # Special meaning for Grafana.
            report.blast_radius.append("dashboards and visual monitoring become unavailable")  # Explain impact.


def build_audit_report() -> dict[str, Any]:  # Main function that builds the full report.
    compose = load_compose(COMPOSE_FILE)  # Load docker-compose.yml.
    reports = parse_compose_services(compose)  # Turn compose services into ServiceReport objects.
    containers = docker_ps()  # Get current Docker containers.
    listening_ports = get_listening_ports()  # Get host listening ports.

    enrich_with_runtime_state(reports, containers)  # Add runtime state to reports.
    detect_risks(reports, listening_ports)  # Add risks to reports.
    calculate_blast_radius(reports)  # Add failure impact notes.

    service_reports = {  # Convert ServiceReport objects into plain dictionaries.
        name: asdict(report)  # Convert dataclass to dict.
        for name, report in sorted(reports.items())  # Sort services alphabetically.
    }

    return {  # Return final full audit report dictionary.
        "project_root": str(PROJECT_ROOT),  # Include project path.
        "compose_file": str(COMPOSE_FILE),  # Include compose file path.
        "services_defined": len(service_reports),  # Count services defined in compose.
        "services_running": sum(1 for report in reports.values() if report.running),  # Count running services.
        "host_listening_ports": sorted(listening_ports),  # Include sorted list of listening ports.
        "services": service_reports,  # Include all service-level reports.
    }


def write_report(report: dict[str, Any], output_file: Path) -> None:  # Writes report to disk.
    output_file.write_text(json.dumps(report, indent=2), encoding="utf-8")  # Convert dict to pretty JSON and save.


def print_summary(report: dict[str, Any]) -> None:  # Prints a human-readable terminal summary.
    print("Pokemon runtime dependency audit")  # Print title.
    print(f"Compose file: {report['compose_file']}")  # Print compose file path.
    print(f"Services defined: {report['services_defined']}")  # Print service count.
    print(f"Services running: {report['services_running']}")  # Print running service count.
    print()  # Print blank line.

    for service_name, service in report["services"].items():  # Loop through service reports.
        status = service["status"]  # Read service status.
        running = "running" if service["running"] else "not running"  # Convert boolean to readable text.
        risks = len(service["risk_flags"])  # Count risk flags.

        print(f"- {service_name}: {running}, status={status}, risks={risks}")  # Print one-line service summary.


def main() -> int:  # Script entrypoint function.
    try:  # Try running the audit.
        report = build_audit_report()  # Build full audit report.
    except Exception as exc:  # Catch any unexpected error.
        print(f"Audit failed: {exc}", file=sys.stderr)  # Print error to stderr.
        return 1  # Return failure exit code.

    write_report(report, OUTPUT_FILE)  # Save report as JSON.
    print_summary(report)  # Print terminal summary.
    print()  # Print blank line.
    print(f"Wrote JSON report to: {OUTPUT_FILE}")  # Tell user where report was written.

    return 0  # Return success exit code.


if __name__ == "__main__":  # Only runs when this file is executed directly.
    sys.exit(main())  # Run main and use its return value as the process exit code.
