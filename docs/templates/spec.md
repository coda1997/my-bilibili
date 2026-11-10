# <Feature title>

Spec ID: `<spec-id>`
Status: proposed; identify selection/architecture-review authority before work.

## User scenario and goal

Who needs what observable outcome, under which starting conditions?

## Scope and non-goals

State the smallest delivered slice, explicit exclusions, and prerequisites.
Name unresolved milestone/dependency decisions rather than assuming acceptance.

## Boundaries and shared contracts

Name the affected modules, owned interfaces/data, and dependency direction.
Specify relevant error, timeout, cancellation, state-transition, late-result,
thread-affinity, and resource-lifetime behavior. Use "not applicable" with a
reason when a concern is genuinely absent. Freeze shared contracts before
parallel implementation; changes return to architecture review and a new run.

## Acceptance criteria

### FEAT-AC-01

Given <starting condition>, when <action>, then <observable successful result>.

### FEAT-AC-02

Given <failure or boundary condition>, when <action>, then <observable safe result>.

## Verification matrix

| AC | Test/command or manual procedure | Fixture and OS | Required evidence |
| --- | --- | --- | --- |
| FEAT-AC-01 | <test name and command> | <controlled fixture / platforms> | <observable result and locator> |
| FEAT-AC-02 | <test name and command> | <failure fixture / platforms> | <observable result and locator> |

Replace every placeholder and use consistent acceptance IDs in tasks.json.
Distinguish compilation from target execution. Record missing OS access as
not-tested, not pass; do not rely on real accounts or private authenticated URLs.

## Risks, rollback, and open decisions

Identify compatibility/security/performance risks as relevant, how to back out
the change without destroying user work, and who resolves blocking questions.

## Review and evidence

Specify required independent review dimensions and any separate human policy
approval. The controller records source/spec identity and sanitized evidence in
its run. State the durable result/report location; do not pre-fill approval or
successful test claims. A proposed spec and this template grant no authority.
