package main

import (
	"context"
	"encoding/json"
	"flag"
	"strings"
	"testing"
)

func TestSuiteSelectionDefaultAndReportLabel(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{{nil, "full"}, {[]string{"-suite", "full"}, "full"}, {[]string{"-suite", "preview"}, "preview"}} {
		flags := flag.NewFlagSet("test", flag.ContinueOnError)
		selected := suiteFlag(flags)
		if err := flags.Parse(tc.args); err != nil {
			t.Fatal(err)
		}
		suite, err := selectSuite(*selected)
		if err != nil || suite != tc.want {
			t.Fatalf("args=%v got=%s err=%v", tc.args, suite, err)
		}
		b, err := json.Marshal(report{Suite: suite})
		if err != nil || !strings.Contains(string(b), `"suite":"`+tc.want+`"`) {
			t.Fatalf("report suite missing: %s %v", b, err)
		}
	}
}

func TestUnknownSuiteRejectedBeforeAppOrFixtureAccess(t *testing.T) {
	for _, name := range []string{"", "Preview", "full ", "unknown"} {
		r := report{Suite: name}
		err := run(context.Background(), "absent-app", t.TempDir(), "absent-fixtures", &r)
		if err == nil || !strings.HasPrefix(err.Error(), "unknown native suite") || r.AppSHA256 != "" || r.ServerStats != nil || len(r.Checks) != 0 {
			t.Fatalf("suite not rejected before launch/access: %+v err=%v", r, err)
		}
	}
}
