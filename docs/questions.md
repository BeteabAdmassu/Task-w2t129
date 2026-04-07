# Business Logic Questions & Decisions

## Decisions Log

1. **Multi-User Concurrency**
   - **Question**: The prompt says "desktop" but doesn't clarify if multiple users can log in simultaneously on the same machine, or if it's strictly single-user.
   - **My Understanding**: Single desktop app, one active user session at a time. The "lock screen" feature implies one person at a time with fast user switching via lock/unlock.
   - **Decision**: Implement single active session. Lock screen returns to login; new user can log in without quitting the app. No concurrent multi-user sessions.
   - **Impact**: If multi-user simultaneous sessions are needed, we'd need session isolation, per-window auth context, and potential database-level row locking for concurrent writes.

2. **Two-Step Approval — Who Approves?**
   - **Question**: Settlement requires "two-step approvals" but the prompt doesn't define who the two approvers are or whether they must be different users/roles.
   - **My Understanding**: Two distinct users must approve, and they cannot be the same person. At least one must be a System Administrator.
   - **Decision**: Implement two-step approval requiring two different users. First approver can be any authorized role; second must be a System Administrator. The user who generated the statement cannot be an approver.
   - **Impact**: If the clinic has only one admin, they'd be blocked from completing approvals. May need a fallback or configurable approval chain.

3. **Work Order Auto-Dispatch Logic**
   - **Question**: The prompt says "auto-dispatches based on trade and workload" but doesn't define how workload is measured or what happens when no technician of the required trade is available.
   - **My Understanding**: Workload = count of open (non-closed) work orders assigned to a technician. Dispatch picks the technician with the matching trade and fewest open orders.
   - **Decision**: Implement round-robin by trade with lowest-open-order-count priority. If no technician with the matching trade exists, flag the work order as "unassigned" for manual dispatch by the Maintenance Supervisor.
   - **Impact**: If workload should factor in estimated hours or SLA urgency weighting, the dispatch algorithm would need to be more sophisticated.

4. **Stored-Value Refund — Definition of "Unused"**
   - **Question**: Refunds are allowed "only within 7 days if unused." Does "unused" mean zero transactions against the stored value ever, or zero transactions since the last top-up?
   - **My Understanding**: "Unused" means the specific stored-value amount being refunded has had no deductions since it was added.
   - **Decision**: Track each stored-value deposit as a discrete record. A refund is allowed only if the deposit was made within 7 days AND no portion of it has been spent (i.e., total stored value balance >= deposit amount, and no deductions after the deposit timestamp).
   - **Impact**: If "unused" means the entire stored-value balance is untouched (not just the most recent deposit), the logic simplifies but becomes more restrictive.

5. **Session Package — Partial Redemption Rule**
   - **Question**: "Partial session redemption not allowed" — does this mean a member must redeem exactly 1 full session at a time, or that they cannot redeem a fraction of a session (e.g., half a session)?
   - **My Understanding**: Each redemption consumes exactly 1 session from the package. You cannot redeem 0.5 sessions or split a session across visits.
   - **Decision**: Session redemption is always 1 whole session per transaction. The remaining_sessions field decrements by 1. No fractional sessions.
   - **Impact**: Minimal — this is the most natural interpretation. If bulk redemption (e.g., redeem 3 at once) is needed, we'd add a quantity parameter.

6. **Membership Freeze — Effect on Expiration**
   - **Question**: Members can freeze/unfreeze, but the prompt doesn't state whether freeze pauses the expiration clock.
   - **My Understanding**: Freezing pauses the expiration countdown. When unfrozen, the expiration date extends by the duration of the freeze.
   - **Decision**: On freeze, record the freeze timestamp. On unfreeze, compute the frozen duration and extend expires_at by that duration. While frozen, no benefits can be redeemed.
   - **Impact**: If freeze does NOT extend expiration (just blocks redemption), the implementation is simpler but less member-friendly.

7. **Signed Export — What Constitutes "Signed"?**
   - **Question**: Exports are "signed CSV/JSON files" but the prompt doesn't specify the signing mechanism (digital signature, HMAC, etc.) or key management.
   - **My Understanding**: Since this is offline with no PKI infrastructure, a practical approach is HMAC-SHA256 using the app's local encryption key.
   - **Decision**: Each exported file includes a detached `.sig` file containing an HMAC-SHA256 signature computed with the local encryption key. The importing system can verify integrity if given the same key.
   - **Impact**: If proper PKI/X.509 digital signatures are required, we'd need certificate management which adds significant complexity.

8. **Business Days for SLA Calculation**
   - **Question**: Normal priority SLA is "3 business days" — but there's no mention of a business-day calendar, holidays, or operating hours.
   - **My Understanding**: Business days = Monday through Friday, excluding weekends. No holiday calendar.
   - **Decision**: Compute SLA deadlines using Mon-Fri as business days. Provide a system configuration option for the admin to define clinic operating hours and holidays in the future, but initially ship with a simple Mon-Fri calculation.
   - **Impact**: If the clinic operates 7 days a week, the SLA would be too generous. If holidays matter, deadlines could be missed. The config option provides a future escape hatch.

9. **Rate Table — Distance-Based Charging Context**
   - **Question**: The prompt mentions "distance in miles," "weight/volume tiering," and "fuel surcharge" for charges and settlement. This seems unusual for a medical clinic — is this for medical courier/delivery services or supply chain logistics?
   - **My Understanding**: This likely covers charging for delivery of medical supplies, specimen transport, or home-visit services where distance-based billing applies.
   - **Decision**: Implement rate tables as a generic charge calculation engine. Each charge line item can reference a rate table and input parameters (distance, weight, volume). The UI will present this as "Charges & Billing" without assuming a specific use case.
   - **Impact**: If this is for a very specific workflow (e.g., ambulance transport billing), the UI and validation might need domain-specific labels and rules.

10. **Offline Update Package Format**
    - **Question**: "Offline updates via imported packages" — the prompt doesn't define the package format, verification, or what gets updated (just the app, or also the database schema?).
    - **My Understanding**: The update package is a self-contained archive containing new Electron app binaries, Go service binary, and database migration files.
    - **Decision**: Update packages are ZIP archives with a manifest.json describing the version, checksums, and migration scripts. The update process: verify checksums, backup current version, apply migrations, replace binaries, restart. Rollback restores the backup.
    - **Impact**: If updates also need to patch PostgreSQL itself or OS-level dependencies, the package format and installer logic become significantly more complex.

11. **Retention Rules for Managed Files**
    - **Question**: Files have "retention rules" but no specifics on default retention period or what triggers deletion.
    - **My Understanding**: Retention is configurable per file category (e.g., work order photos: 2 years, audit exports: 7 years). Expired files are soft-deleted and purged by a background cleanup.
    - **Decision**: Implement a configurable retention policy table (entity_type, retention_days). A daily background job marks files past retention for cleanup. Admin can override per-file. No files are auto-deleted without admin confirmation on the first run.
    - **Impact**: If retention must comply with specific healthcare regulations (HIPAA, state laws), the defaults would need to be reviewed by compliance.

12. **Competency Review in Learning Module**
    - **Question**: The Learning Coordinator role includes "competency review" but the prompt doesn't detail what a competency review entails — is it quiz-based, supervisor sign-off, or just content read tracking?
    - **My Understanding**: Competency review is a manual process where the Learning Coordinator marks staff as competent on specific knowledge points after review (e.g., after training, observation, or assessment).
    - **Decision**: Implement a simple competency tracking table: staff member + knowledge point + status (pending, competent, needs_improvement) + reviewer + review date + notes. No automated quizzing.
    - **Impact**: If automated assessments or quiz functionality is expected, this would require a significant additional feature (question banks, scoring, pass/fail thresholds).
