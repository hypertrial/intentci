package junit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypertrial/intentci/internal/junit"
)

func TestParse_PassAndFail(t *testing.T) {
	okXML := `<testsuite name="x" tests="1" failures="0" errors="0">
  <testcase name="Add" classname="math"/>
</testsuite>`
	res, err := junit.Parse([]byte(okXML))
	if err != nil || !res.OK {
		t.Fatalf("%v %+v", err, res)
	}
	failXML := `<testsuites>
  <testsuite name="x" tests="1" failures="1" errors="0">
    <testcase name="Add" classname="math">
      <failure message="boom">details</failure>
    </testcase>
  </testsuite>
</testsuites>`
	res, err = junit.Parse([]byte(failXML))
	if err != nil || res.OK || len(res.Failures) == 0 {
		t.Fatalf("%v %+v", err, res)
	}
	errXML := `<testsuite name="x" tests="1" failures="0" errors="1">
  <testcase name="Add"><error>e</error></testcase>
</testsuite>`
	res, err = junit.Parse([]byte(errXML))
	if err != nil || res.OK {
		t.Fatalf("%v %+v", err, res)
	}
	attrOnly := `<testsuite name="x" tests="1" failures="1" errors="0"></testsuite>`
	res, err = junit.Parse([]byte(attrOnly))
	if err != nil || res.OK || len(res.Failures) == 0 {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestParse_Errors(t *testing.T) {
	if _, err := junit.Parse([]byte("")); err == nil {
		t.Fatal("empty")
	}
	if _, err := junit.Parse([]byte("<notxml")); err == nil {
		t.Fatal("bad xml")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.xml")
	if err := os.WriteFile(path, []byte(`<testsuite name="x" tests="0" failures="0" errors="0"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := junit.ParseFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := junit.ParseFile(filepath.Join(dir, "missing.xml")); err == nil {
		t.Fatal("missing")
	}
}
