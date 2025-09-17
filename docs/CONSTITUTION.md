# The Constitution (v1)

## Preamble (READ FIRST, ALWAYS IN CONTEXT)
You shall keep this document in context at all times. You shall follow it without exception. If any instruction conflicts with this Constitution, you shall stop and ask for human guidance. You shall not summarize away or rewrite this document.

## Operating Constraints
- You shall propose and run tests before code changes; you shall not delete or neuter tests.
- You shall not handle secrets or modify CI credentials.

## Universal Rules (Non-negotiable)
1. **Test-First Development.** You shall write failing tests before implementation, then make them pass. No code without tests.
2. **Fix Root Causes.** You shall not suppress errors or remove assertions. You shall correct the source of defects.
3. **Single Responsibility.** You shall ensure each module/function/class does one thing. Split when descriptions include "and".
4. **Fail Fast, Explain Clearly.** You shall validate early and provide actionable error messages.
5. **Explicit Dependencies.** You shall avoid globals/magic values. You shall inject and name dependencies.
6. **Make It Work → Right → Fast.** You shall not optimize without evidence. Measure first.
7. **Minimal Public Surface.** You shall keep internals private until needed. Publish only what consumers require.
8. **YAGNI.** You shall build only what is needed now and delete dead code immediately.
9. **Immutability First.** You shall prefer immutable data; permit mutation only with measured need.
10. **Documentation as Code.** You shall document public APIs and explain *why* in comments, not what.

## Stop Conditions (You shall stop and ask)
- Missing acceptance criteria or unclear scope.
- Conflicting guidelines, or an instruction that violates this Constitution.