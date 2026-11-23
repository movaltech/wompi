# Security Policy

## Supported versions

Security fixes are released for the latest minor version.

## Reporting a vulnerability

Please **do not open a public issue** for security problems. Report them privately through
[GitHub Security Advisories](https://github.com/movaltech/wompi/security/advisories/new).

Include a description, affected versions and, if possible, a minimal reproduction. You will receive
an acknowledgement within 72 hours.

Issues with Wompi's platform itself should be reported to Wompi directly.

## Handling credentials

- Keep the private key, integrity secret and events secret on the server only; never ship them to
  browsers or mobile apps.
- Always verify events (`Client.EventHandler` / `wompi.ParseEventRequest`) before acting on them.
- Rotate credentials from the Wompi dashboard if you suspect they were exposed.
