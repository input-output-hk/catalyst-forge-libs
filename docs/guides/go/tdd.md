Here’s a **small, TDD-only** cheat sheet for **LLM agents writing Go**—no style or formatting tips, just the principles and gotchas that matter.

# Go TDD essentials (for LLMs)

1. **Work in tiny R-G-R cycles.** Keep a short “test list,” write a failing test for the next behavior, make it pass with the *simplest* code, then refactor. Use Beck’s green-bar strategies: *Fake It*, *Obvious Implementation*, *Triangulation*. ([martinfowler.com][1], [Pearson IT Certification][2], [stanislaw.github.io][3])

2. **Test behavior, not implementation.** Aim tests at public behavior (API/contracts). Avoid coupling to private structure—those tests become brittle. Cooper’s “Where did TDD go wrong?” and Fowler’s guidance on mockist vs classical TDD are the lodestars here. ([InfoQ][4], [martinfowler.com][5])

3. **Shape the design for testability.** Isolate domain logic from I/O with **ports & adapters** (hexagonal). In Go, prefer *accepting interfaces at the boundary and returning concrete types*—define the interface where it’s consumed. ([alistair.cockburn.us][6], [dave.cheney.net][7])

4. **Choose doubles on purpose.** Default to **fakes/in-memory** for external systems; reserve **mocks** for places where interaction protocols matter (e.g., “must call X with Y”). Over-mocking couples tests to internals. ([martinfowler.com][5])

5. **Keep tests deterministic and hermetic.** No real time, sleeps, randomness, network, or disk in unit-level tests; inject time/entropy and keep scope in-process. Treat flakiness as a bug. Google’s “small test” constraints and Fowler’s essay on non-determinism are the benchmarks. ([abseil.io][8], [martinfowler.com][9])

6. **Handle concurrency deliberately.** Prefer explicit synchronization points over timing. Continuously run with the **Go race detector** and treat any finding as a defect in the code or the test. ([Go.dev][10])

7. **Bias to many small tests; few large ones.** Most coverage should come from fast, behavior-focused unit tests; add a thin layer of integration/acceptance checks for wiring. Avoid piling on end-to-end tests. ([martinfowler.com][11], [Google Testing Blog][12])

8. **Use property/fuzz tests to extend confidence.** After you have exemplar tests for a behavior, add Go’s built-in **fuzzing/property checks** to explore edge cases (great for parsers, encoders, normalization). Keep failing inputs as seeds. ([Go.dev][13])

9. **Refactor on green; fix over-specification.** Refactor mercilessly to remove duplication and clarify names. If a legitimate refactor breaks tests, those tests were coupled to implementation—rewrite them to target behavior. ([jamesshore.com][14])

10. **For legacy code, characterize first.** Before changing untested areas, write **characterization tests** to pin current behavior and cut seams to break hard dependencies; then proceed with TDD for the new behavior. ([Wikipedia][15], [martinfowler.com][16])

11. **Optimize the loop, not dogma.** Evidence suggests TDD’s benefits come chiefly from **fine-grained, uniform steps** and steady refactoring—not merely from “test-first.” Keep steps small and regular. ([arXiv][17])

---

If you want, I can turn this into a tiny “**TDD planner prompt**” your agents prepend (it would ask for the next behavior, seam choices, and the minimal fake to keep tests hermetic).

[1]: https://martinfowler.com/bliki/TestDrivenDevelopment.html?utm_source=chatgpt.com "Test Driven Development"
[2]: https://ptgmedia.pearsoncmg.com/images/9780321146533/samplepages/0321146530.pdf?utm_source=chatgpt.com "Test-Driven Development"
[3]: https://stanislaw.github.io/2016-01-25-notes-on-test-driven-development-by-example-by-kent-beck.html?utm_source=chatgpt.com "Notes on \"Test-Driven Development by Example\" by Kent Beck"
[4]: https://www.infoq.com/presentations/tdd-original/?utm_source=chatgpt.com "TDD: Where Did It All Go Wrong?"
[5]: https://martinfowler.com/articles/mocksArentStubs.html?utm_source=chatgpt.com "Mocks Aren't Stubs"
[6]: https://alistair.cockburn.us/hexagonal-architecture?utm_source=chatgpt.com "hexagonal-architecture - Alistair Cockburn"
[7]: https://dave.cheney.net/2016/08/20/solid-go-design?utm_source=chatgpt.com "SOLID Go Design"
[8]: https://abseil.io/resources/swe-book/html/ch11.html?utm_source=chatgpt.com "Testing Overview"
[9]: https://martinfowler.com/articles/nonDeterminism.html?utm_source=chatgpt.com "Eradicating Non-Determinism in Tests"
[10]: https://go.dev/blog/race-detector?utm_source=chatgpt.com "Introducing the Go Race Detector"
[11]: https://martinfowler.com/articles/practical-test-pyramid.html?utm_source=chatgpt.com "The Practical Test Pyramid"
[12]: https://testing.googleblog.com/2015/04/just-say-no-to-more-end-to-end-tests.html?utm_source=chatgpt.com "Just Say No to More End-to-End Tests"
[13]: https://go.dev/doc/security/fuzz/?utm_source=chatgpt.com "Go Fuzzing"
[14]: https://www.jamesshore.com/v2/books/aoad2/test-driven_development?utm_source=chatgpt.com "AoAD2 Practice: Test-Driven Development"
[15]: https://en.wikipedia.org/wiki/Characterization_test?utm_source=chatgpt.com "Characterization test"
[16]: https://martinfowler.com/bliki/LegacySeam.html?utm_source=chatgpt.com "Legacy Seam"
[17]: https://arxiv.org/abs/1611.05994?utm_source=chatgpt.com "A Dissection of the Test-Driven Development Process: Does It Really Matter to Test-First or to Test-Last?"
