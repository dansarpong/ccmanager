# Workdir memory

<!-- maintainer note: this comment must be stripped from "what Claude sees" -->

Real instruction one.

Import chain starts here: @../../../imports/imp_a.md

A circular import: @../../../imports/cyc_x.md

An external import: @../../../../configroot/CLAUDE.md

To mention a path literally, write `@README` in backticks and it stays text.

```text
This fenced block mentions @../../../imports/imp_a.md but it must NOT import.
<!-- this comment is inside a code fence and must be preserved -->
```

Final instruction.
