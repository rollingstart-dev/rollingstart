# Security Policy

## Reporting a vulnerability

Please report security issues through GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
on this repository, or by email to brandt@kurowski.net.

Please don't open a public issue for a vulnerability.

## What Rolling Start executes

Reviewers should understand the trust boundary, because Rolling Start runs code
by design.

**Instance-declared commands and operations are shell**, and Rolling Start runs
them. They are written by a human instance author, committed to the target
repository, and code-reviewed like anything else there. Operations marked
`destructive` prompt before running.

**Generated verifiers are not shell.** Task generation emits test files and
command *selections* interpreted by the harness — never a script. The residual
risk is equivalent to running the target repository's own test suite, which the
learner already does.

**Rolling Start does not orchestrate your environment.** It never starts
services, manages containers, or changes system state outside the working copy
and its own state directory.

**Learner profiles stay local.** They are written under `.rollingstart/profile/`,
which carries its own `.gitignore`, and are never transmitted anywhere.

## Model endpoints

The model endpoint is configurable. Rolling Start can be pointed at Bedrock,
Azure, or an internal gateway, so source never has to leave your network.
