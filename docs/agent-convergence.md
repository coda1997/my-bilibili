# Agent findings, scope, and convergence

Status: the owner approved this scoped policy replacement on 2026-11-16 after
the read-only inventory. It governs new assignments and this documentation
change. It replaces the general P0/P1 blocking rule and the former allowance
of three repair/re-verification rounds. Severity remains useful triage metadata.
It grants no commit, remote issue/comment, PR, merge, or release authority.
Keep the existing required checks and explicit human merge decision.

Historical reviews, published assignments, frozen inputs and closed runs retain
their original meaning. An already frozen task, including M2 spec 0015, keeps
its recorded policy until a newly authorized continuation packet explicitly
records the transition, prior review/repair history and remaining budget. Never
edit the old packet or reset a counter by switching policy. Changing that task's
contract still follows the contract-review/freeze procedure.

## R1. Two finding tiers; a closed blocking list

Every review finding is `BLOCKING [class N]` or `ADVISORY`. A BLOCKING finding
must identify evidence and one of these seven classes:

1. **Correctness:** wrong result, crash, hang or race on a reachable path,
   including observable state, cancellation or resource-lifetime failures
   against the assigned behavior.
2. **Security/privacy:** an evidenced credential, personal-data, authentication
   or isolation gap.
3. **Data loss/corruption:** a reachable loss or corruption of user data or
   required evidence.
4. **Untested new mechanism:** this change adds a guard, branch or limit, and
   no regression check fails when that mechanism is reverted. Name the new
   mechanism and the missing discriminating test; requests for additional
   cases around an already tested mechanism are advisory.
5. **Required verification:** a previously agreed required check fails, required
   acceptance evidence is missing, or a check was weakened without justification
   and the required approval. Cite the existing requirement and actual gap;
   do not invent a new gate for naming, wording or stylistic preferences.
6. **Operational breakage:** a known consumer of an interface, CLI, evidence
   format, automation output, dashboard or alert is broken by the change and
   no follow-up owner/action is named. Identify the consumer and dependency.
7. **Unsupported fix:** the proposed fix depends on an unobserved cause or an
   undocumented contract. Identify that dependency; speculative suspicions
   about otherwise working code are investigation notes, not blockers.

Everything else is ADVISORY, including extra test cases, wording, naming,
structure, refactors and rule violations without demonstrated harm. Severity
alone does not determine the tier. Only BLOCKING findings can hold technical
approval/merge readiness or require another change in this task. High-severity
claims await the independent ruling in R6; approval never substitutes for
separate human authority or actual required checks.

## R2. Finish advisory threads

The author either addresses an ADVISORY in an in-scope change already being
made, or records its destination: an existing issue, the run's single follow-up
issue, or the run's follow-up list with the R5 reason for not filing. That answer
closes the advisory disposition without another approval or repair round.
Closing a remote thread still requires remote-action authority. Only the
assigned independent reviewer closes BLOCKING findings technically.

A reviewer must not raise an answered ADVISORY again unless the lines it cites
changed. A changed candidate hash alone is insufficient. New evidence of a
closed-list harm is a distinct finding linked to the prior record; it cannot
be a renamed request for the same advisory change.

## R3. Lock scope

Before implementation, identify the original goal, done criteria, assigned
issue/bug class and write paths. The diff covers only that scope. A bug-class
assignment must name its affected behavior and paths; it is not permission to
sweep and repair every similar instance in the repository.

New problems outside that scope are recorded and triaged under R5, never
automatically added to the current work, even if real or severe. If an external
problem makes the original goal unsafe or impossible, report the evidenced
dependency and stop the affected work for a scope decision. Do not repair it
without an authorized contract/scope change. Review of a repair covers its
responses, changed lines and affected behavior; it does not start a new sweep.

## R4. Bound review and automatic repair

- At most **3 formal review rounds per change/task**, including the initial
  review as round 1. Consolidate all review dimensions for that round. A
  submitted candidate or evidence-backed response answering every finding
  starts the next round; an advisory disposition under R2 counts as answered.
  Clarification messages do not start a round. A push alone does not either.
- At most **3 automatic fix iterations per task**. An iteration is one attempted
  repair and its verification; count failed attempts too. Stop when the same
  failure occurs **3 times**, even if hashes, agents or error wording differ.
  Track the underlying failed assertion/behavior. The first reached limit
  applies; the fix budget does not permit a fourth review.
- Finish as soon as required checks pass, no BLOCKING finding affecting the
  assigned scope or an evidenced prerequisite remains open, and every
  high-severity claim has an independent ruling. Do not use up the budget
  for its own sake. Answered ADVISORY items never hold completion or merge
  readiness. Actual merge still requires the existing explicit human decision.
- At the final round, unresolved BLOCKING findings stop automatic work. There
  is no uncounted "final verification" or new reviewer sweep after round 3.
  Do not hide further repair/review in a new run, agent, task or candidate.
  Record cumulative counts across those changes.
- Escalate earlier for a needed human decision, recurring failure without new
  evidence, or reviewer disagreement without new evidence. Supply goal,
  task/spec/candidate, open IDs, attempts and evidence by round, competing
  conclusions, options and a recommended next action. Record `needs-human` in
  an assigned run. Only explicit human direction with scope and additional
  allowance resumes the stopped loop; preserve its history.

The current `cmd/dev` checker/recorder has no agent repair/review/issue loop;
these limits are procedural. Any future such loop must enforce them in code
before use, with persistent counters and a trusted policy baseline. The fixed
check-step traversal, event-storage limit and media polling are not agent loops.
Candidate-controlled policy cannot approve itself.

## R5. Record first; file selectively and in batches

For each logical run/sweep, the controller owns one finding inventory and one
filing budget shared by its workers, review rounds and continuation runs. List
all findings first, then triage the complete list in one pass. A later bounded
round may add newly evidenced findings to that same inventory; it gets no new
filing budget. Finding IDs/local notes are not new remote issues.

Apply these rules before any authorized filing:

1. **Threshold:** file only evidenced user impact, data loss, operational risk,
   blocked correct behavior or hidden real errors. Do not file cosmetic changes,
   speculative edge cases, style changes to working code or unsupported guesses.
2. **Dedupe:** search open issues and the local known-issue/finding inventory.
   Record search terms/results. If an issue matches, add the new evidence to it
   with one consolidated comment instead of opening another. Without search
   access or remote-action authority, record a pending draft; do not claim
   dedupe or publication succeeded.
3. **Cap:** create at most **5 new issues per logical run**, counting its single
   advisory follow-up issue. Rank qualifying items by demonstrated impact.
   Record everything not filed and why, including items over the cap. A new
   agent, resumed run or split task does not refresh this budget.
4. **Batch:** put all qualifying ADVISORY findings in **one follow-up issue**
   (reuse a matching existing issue when possible), not one issue per comment.
   Keep below-threshold advisories in the same local follow-up list with their
   no-filing reasons; batching does not bypass the threshold.
5. **Fix follow-up:** a fix opens a systemic follow-up only when the bug was
   introduced by an earlier fix, missing tests form a whole category, evidence
   suggests the same pattern exists elsewhere, or the root cause is a chain.
   State the evidence for that condition; threshold/dedupe/cap still apply.
   Otherwise its PR states `No systemic follow-up warranted.` An unrelated
   evidenced discovery may still qualify for the ordinary backlog; record it
   separately from claims about the fix's root cause and never implement it
   under the current task.

Reusable issue-filing prompt / controller return format:

> Read R1/R3/R5/R7/R8 before triage. Return the complete inventory before any
> authorized remote write. For each ID supply tier/class, observed impact and
> evidence, in/out of scope, duplicate search/result, existing issue or proposed
> batch, owner/next action, and filed/deferred/not-filed reason. Include prior
> issue count, planned new count, the single advisory batch destination, and
> any systemic follow-up justification. After authorized writes, record actual
> issue/comment identifiers and the updated count. Recording never dispatches
> implementation; issue creation, comments and closure need their own authority.

## R6. Independent rulings on high severity

Treat a P0/P1 claim, or a finding described as high/critical severity, as a
high-severity claim regardless of the author's preferred tier. A reviewer
independent of implementation, or a human, supplies one line per such finding:

```text
<finding id>: BLOCKING | not BLOCKING: <one-line reason>
```

Choose one verdict, not both; a BLOCKING verdict cites its R1 class and evidence.
Bind the ruling to the exact candidate and contract. The author's "follow-up"
is not a ruling. No merge readiness while a high-severity claim is unruled.
For ordinary BLOCKING closure use the assigned reviewer and the existing
reviewer-succession procedure; do not replace a disagreeing reviewer to get a
preferred answer. A disposition permitted by this policy does not prove a
missing required acceptance result.

## R7. Observe the cause and contract

If the cause or relied-on contract cannot be observed in a log, reproduction,
test, source behavior or documented API/contract, return a question or bounded
investigation note. Do not guess a fix or file a speculative issue. A reproduced
symptom can qualify for the R5 backlog even when its cause is unknown; label
the unknown cause and do not claim a causal repair.

Every fix PR states `Observed vs inferred:` and `Contract relied on:` with
concrete evidence/contract references. Preserve the difference between verified
behavior, plausible explanations and untested assumptions. For a new guard,
branch or limit, provide the R1 class-4 discriminating regression evidence.

## R8. Turn recurrence into one structural proposal

On the second evidenced occurrence of the same kind (file conflict, bug class
or flaky area), link the earlier occurrence and stop filing instance issues.
Propose one structural fix, such as a generator, lint, canonical format or shared
helper, justified by the observed recurrence. Reuse an existing structural
issue or apply R5 before creating one; a proposal does not authorize implementation.

With issue-management authority, consolidate/close instance issues against the
structural issue as duplicates or tracked there, preserving their evidence and
ownership. Without that authority, record the proposed links/closures. Do not
mark an unresolved defect fixed, close a review BLOCKING finding, waive a check
or declare the current task done merely by moving its issue. Relevant blockers
still need independent disposition.

## R9. Re-orient before done and after context reset

Before claiming done, and on resuming after a context reset, state:

- Original goal and locked scope.
- Each done criterion with candidate-specific evidence or its remaining gap.
- What was observed versus inferred, and the contract relied on.
- Current review/fix/failure counts and issue budget from the retained ledger.
- Next action (or none for this completed scope).
- **Found, not done:** everything outside the original goal, with its inventory,
  issue or investigation-note destination and no-filing reason where applicable.

Use the assignment, review, PR and handoff templates to carry this information.
Checkpoint significant progress and next actions in an assigned controller run;
workers return evidence to its owner. No run outcome or self-check is independent
approval, and an unavailable required result remains a gap.
