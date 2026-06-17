package main

import "testing"

func TestCheckBashSafe(t *testing.T) {
	for _, cmd := range []string{"pwd", "ls", "ls ./", "cat README.md", "grep foo README.md", "grep -R foo .", "find . -name *.go", "find ./src -type f", "go test ./..."} {
		t.Run(cmd, func(t *testing.T) {
			res, err := CheckBash(cmd, "")
			if err != nil {
				t.Fatal(err)
			}
			if res.Risk != RiskSafe {
				t.Fatalf("risk=%v reason=%s", res.Risk, res.Reason)
			}
		})
	}
}

func TestCheckBashNeedsConfirm(t *testing.T) {
	for _, cmd := range []string{"rm file.txt", "rm -rf tmp", "mv a.txt b.txt", "cp a.txt b.txt", "mkdir tmp", "touch file.txt", "go mod tidy", "npm install", "find . -name *.tmp -delete", "find . -exec rm {} \\;", "xargs rm"} {
		t.Run(cmd, func(t *testing.T) {
			res, err := CheckBash(cmd, "")
			if err != nil {
				t.Fatal(err)
			}
			if res.Risk != RiskNeedsConfirm {
				t.Fatalf("risk=%v reason=%s", res.Risk, res.Reason)
			}
		})
	}
}

func TestCheckBashFormerlyDeniedNeedsConfirm(t *testing.T) {
	for _, cmd := range []string{"cat /etc/passwd", "rm ../file", "rm -rf /", "mv file /tmp/file", "echo hi > ../x", "cd ..", "sudo rm file", "bash -c cat", "sh -c rm", "echo $(cat /etc/passwd)", "find /etc -name passwd", "find . -exec cat /etc/passwd \\;", "find . -exec sh -c rm \\;"} {
		t.Run(cmd, func(t *testing.T) {
			res, err := CheckBash(cmd, "")
			if err != nil {
				t.Fatal(err)
			}
			if res.Risk != RiskNeedsConfirm {
				t.Fatalf("risk=%v reason=%s", res.Risk, res.Reason)
			}
		})
	}
}

func TestCheckBashEmptyDenied(t *testing.T) {
	res, err := CheckBash("   ", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Risk != RiskDenied {
		t.Fatalf("risk=%v reason=%s", res.Risk, res.Reason)
	}
}

func TestCheckBashWorkdir(t *testing.T) {
	res, err := CheckBash("pwd", "../")
	if err != nil {
		t.Fatal(err)
	}
	if res.Risk != RiskNeedsConfirm {
		t.Fatalf("risk=%v reason=%s", res.Risk, res.Reason)
	}
}
