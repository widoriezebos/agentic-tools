# taskrun

Build the tool described in `spec.md`.

Start here. This repository is otherwise empty: there is no implementation yet
and no `pom.xml`, and `src/` is where your code goes.

The environment has no network. A Maven repository is already populated with
what a plain Java build needs, including JUnit 5, so every Maven command must
use `-o`. A dependency that is not already there cannot be resolved.

When you are done, this file should carry the `Test command:` line `spec.md`
requires.
