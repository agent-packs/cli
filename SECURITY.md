# Security Policy

## Supported Versions

Agent Packs is pre-1.0. Security fixes are made on the `main` branch and included in the next tagged release.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately.

- Open a private GitHub security advisory for this repository when available.
- If that is not available, contact the repository owner directly through GitHub.

Please include:

- affected version or commit
- operating system and install method
- reproduction steps
- expected and observed behavior
- any relevant pack, plugin, source URL, lockfile, or receipt

Do not open a public issue for vulnerabilities until a fix or mitigation is available.

## Security Scope

Security-sensitive areas include:

- native plugin command execution
- release artifacts and checksums
- registry source resolution
- remote registry update behavior
- lockfile, receipt, and rollback handling
- policy checks and trust metadata

Agent Packs previews native plugin commands by default. Users should pass `--execute-plugins` only for sources they trust.
