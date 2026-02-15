# Security Policy

## Supported Versions

Only the latest release receives security updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

Please **do not** report security vulnerabilities through public GitHub issues.

Instead, open a [GitHub Security Advisory](https://github.com/Alijeyrad/gotalk-dictation/security/advisories/new) on this repository. This keeps the report private until a fix is available.

Include as much of the following as possible:

- Description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept
- Affected version(s)
- Any suggested mitigations (optional)

You can expect an acknowledgement within **72 hours** and a status update within **7 days**. If the vulnerability is confirmed, a patched release will be made as soon as practical and you will be credited in the release notes (unless you prefer otherwise). If the report is declined, you will receive an explanation.

## Scope

Areas of particular interest for this application:

- **Credential exposure** — `GOOGLE_APPLICATION_CREDENTIALS` / GCloud ADC handling
- **Audio data leakage** — PCM/FLAC payloads sent to speech API endpoints
- **Transcript injection** — malicious speech input influencing `xdotool` commands
- **Config file** — `~/.config/gotalk-dictation/config.json` permission or injection issues
