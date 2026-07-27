---
id: REQ-JAVA-001
title: Java calculator adds integers
status: active
priority: required
owners:
  - example-maintainers
applies_to:
  paths:
    - "src/**/*.java"
---

# Intent

The Java calculator must add two integers correctly.

# Obligations

```yaml
- id: OBL-JAVA-001
  statement: The Java sources compile and the test program passes.
  required: true
  verify:
    provider: command
    id: java-test
    run: "mkdir -p build && javac -d build src/main/java/example/Calculator.java src/test/java/example/CalculatorTest.java && java -cp build example.CalculatorTest"
    inherit_environment:
      - HOME
      - JAVA_HOME
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
```
