# Security Policy

Thanks for helping to make Foragd a safe and useful application for everyone.

## Supported Versions

Only the latest released version of Foragd will be supported with security updates.

## Reporting a Vulnerability

If you discover a security vulnerability, we'd appreciate a non-public disclosure. systemd developers can be contacted
privately by creating a new [Security Advisory on
GitHub](https://github.com/systemd/immanent-tech/foragd/advisories/new) (preferred) or via the
[security@immanent.tech](mailto:security@immanent.tech) email address.

**Please do not report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

### Information to Include

Please include as much of the information listed below as you can to help us
better understand and resolve the issue:

- The type of issue (e.g., buffer overflow, SQL injection, or cross-site scripting)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

This information will help us triage your report more quickly.

### Requirements for a Valid Report

- Please ensure the issue is reproducible on main.
- Please ensure a fully working, end-to-end reproducer is provided.
- Please ensure the reproducer is real-world and not simulated or abstracted.
- Please ensure the reproducer demonstrably violates a security boundary.
- Please understand that most of our maintainers are volunteers and already have a heavy review burden. While we will
  try to triage and fix issues in a timely manner, we cannot guarantee any fixed timeline for issue resolution.
- While modern industry practices around coordinated disclosures encourage public disclosure to avoid vendors
stonewalling researchers, we are an open source project that would gain little from needlessly stonewalling researchers.
We thus kindly request that reporters do not publicly disclose issues they have reported to us before an agreed-to
disclosure date.
