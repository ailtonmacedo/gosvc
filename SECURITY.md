# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| 1.x | Yes |
| 0.x | No |

## Reporting a vulnerability

Use GitHub private vulnerability reporting when the repository is published. Until a final repository is selected, contact the maintainers through a private channel and include:

- affected version;
- reproduction steps;
- impact assessment;
- suggested mitigation, when available.

Do not disclose the issue publicly before a fix is available.

## Plugin trust model

Plugins are external executables. `gosvc` verifies declared checksums and runs plugins against temporary copies, but this is not an operating-system sandbox. Install plugins only from trusted sources.
