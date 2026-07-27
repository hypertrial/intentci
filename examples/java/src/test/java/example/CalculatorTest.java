package example;

public final class CalculatorTest {
    public static void main(String[] args) {
        if (Calculator.add(2, 3) != 5) {
            throw new AssertionError("2 + 3 must equal 5");
        }
        System.out.println("intentci-ok");
    }
}
