# Java example

This example deliberately uses only the JDK, so no build service or dependency
download is required.

```bash
mkdir -p build
javac -d build src/main/java/example/Calculator.java src/test/java/example/CalculatorTest.java
java -cp build example.CalculatorTest
intentci compile --strict
intentci verify --all --no-git
```
