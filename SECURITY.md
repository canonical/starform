# Security

## Risks

- The user's script uses a large amount of system memory when executed (caused by improperly-bounded builtins)
- The user's script takes a very long time to run, starving other components of execution time (caused by improperly-bounded builtins)
- The user's script accesses data outside of the sandbox (caused by improperly-bounded builtins)
- Sensitive data is exposed

## Good practices

- Use the `startest` framework to test safety properties of all exposed builtins
- Bound the size of a user's script sets
- If sensitive values must be exposed:
  - Limit users who can access it by adding permission checks in related builtins
  - Use custom types (which implement `starlark.Value`) to avoid sensitive data accidentally ending up in logs
  - Restrict access to logs

## Deployment checklist

- [ ] All exposed builtins abide by all safety properties defined in Starlark (memory usage is accounted, execution time is bounded etc.)
- [ ] Script set size is externally bounded
- [ ] Secrets or properties of secrets are not be visible in logs (e.g. via `print(secret)`)

## Cryptographic functionality

Starform uses SHA-384 to create hashes of input files used as the keys in its default script cache.

## Reporting a vulnerability

Please provide a description of the issue, the steps you took to
create the issue, affected versions, and, if known, mitigations for
the issue.

The preferred way to report a security issue is through
[GitHub's security advisory for this project](https://github.com/canonical/starform/security/advisories/new). See
[Privately reporting a security
vulnerability](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
for instructions on reporting using GitHub's security advisory feature.

The [Ubuntu Security disclosure and embargo
policy](https://ubuntu.com/security/disclosure-policy) contains more
information about how can contact us, what you can expect when you contact us,
and what we expect from you.
