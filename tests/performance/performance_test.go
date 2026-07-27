package performance_test

import (
	"os"
	"os/exec"
	"testing"
)

func BenchmarkV1CLIStartup(b *testing.B) {
	binary := os.Getenv("INTENTCI_BENCHMARK_BINARY")
	if binary == "" {
		b.Skip("INTENTCI_BENCHMARK_BINARY is not set")
	}
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if err := exec.Command(binary, "version").Run(); err != nil {
			b.Fatal(err)
		}
	}
}
