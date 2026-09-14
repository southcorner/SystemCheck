# SystemCheck — Privacy, Consent & Legal

**Read this before deploying SystemCheck.** Monitoring people at work is lawful in
many places *only* when done transparently, proportionately, on company-owned
equipment, with notice and (often) consent, and with bounded retention. Requirements
vary by country and state. This document is guidance, not legal advice — confirm your
obligations with counsel for every jurisdiction where your staff work.

## Principles SystemCheck is built around

1. **Transparency** — staff are told what is collected, why, how long it is kept, and
   who can see it. The agent shows a visible indicator whenever it is active.
2. **Consent** — an agent does not begin collecting until a `consent_record` exists
   for that user/machine (see below). This is enforced in the policy layer.
3. **Proportionality & data minimization** — collect the least needed for the stated
   purpose. Region blur, per-employee exclusions, clipboard off by default, and
   configurable capture intervals all serve this.
4. **Purpose limitation** — data is for security and legitimate business oversight,
   not for unrelated purposes.
5. **Retention limits** — every data type has a retention window; a purge job deletes
   data past it automatically.
6. **Access control & accountability** — RBAC + MFA restrict who can view data, and an
   append-only audit log records every access.
7. **Data-subject rights** — support access/export requests (DSAR) and correction/
   deletion where the law requires.

## Consent workflow

- Publish a monitoring policy to staff (template below).
- Record each person's acknowledgement as a `consent_record` (who, what policy
  version, when, method).
- The agent's policy pull returns `active: false` until a valid consent record exists;
  the agent then stays idle (indicator shows "monitoring not active").

## What to exclude by default

- Known personal/sensitive domains: banking, health, government, personal webmail.
- Password managers and personal apps.
- Screenshot blur for credential fields where detectable.
- Clipboard capture: **off by default**; enable only with a documented justification.

## Retention defaults (tune to your legal advice)

| Data | Default retention |
|---|---|
| Screenshots | 30 days |
| App usage | 90 days |
| Domains / transfers | 90 days |
| Security events | 180 days |
| Audit log | 365 days (or per legal hold) |

## Employee notice — template (adapt with counsel)

> Your employer uses SystemCheck to monitor company-owned devices for security and
> legitimate business purposes. On this device we collect: periodic screenshots,
> which applications are used, files downloaded/uploaded, websites' domains accessed,
> and security events (e.g. USB usage, printing). We do **not** intercept the content
> of encrypted traffic or log your keystrokes. Data is stored on our own servers,
> access is restricted and logged, and it is deleted on the schedule in our monitoring
> policy. Personal/sensitive sites (banking, health) are excluded. A visible indicator
> shows when monitoring is active. Questions or access requests: [contact].

## Jurisdiction notes (non-exhaustive)

- **EU/UK (GDPR/UK GDPR):** need a lawful basis (usually legitimate interests, with a
  balancing test / DPIA), transparency, minimization, and data-subject rights.
- **US:** varies by state; several require notice, some two-party consent for certain
  captures. Check state law where each employee works.
- **India (DPDP Act):** notice + consent and purpose limitation obligations.

Always confirm locally before deployment.
