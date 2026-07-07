package common

import (
	"strings"
	"testing"
)

// TestIsRunningInContainer_EnvSignal covers the container-detection heuristic:
// on a clean host (no /.dockerenv, no /proc, no runtime env vars) it must report
// false, and a well-known runtime env var flips it to true.
func TestIsRunningInContainer_EnvSignal(t *testing.T) {
	// Clean slate for the env-var probe (methods 1/2 read files absent on this host).
	for _, k := range []string{"KUBERNETES_SERVICE_HOST", "DOCKER_CONTAINER", "container"} {
		t.Setenv(k, "")
	}
	if IsRunningInContainer() {
		t.Skip("host actually reports as a container (/.dockerenv or /proc signal); env-flip assertion not meaningful here")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	if !IsRunningInContainer() {
		t.Error("KUBERNETES_SERVICE_HOST set must be detected as a container")
	}
}

// TestSeconds2Time_AllUnits drives every unit branch (year/month/day/hour/
// minute/second) with a value that carries a remainder in each.
func TestSeconds2Time_AllUnits(t *testing.T) {
	// 1y + 1mo + 1d + 1h + 1m + 1s in the function's own unit sizes.
	n := 31104000 + 2592000 + 86400 + 3600 + 60 + 1
	s := Seconds2Time(n)
	for _, unit := range []string{"年", "个月", "天", "小时", "分钟", "秒"} {
		if !strings.Contains(s, unit) {
			t.Errorf("Seconds2Time(%d)=%q missing unit %q", n, s, unit)
		}
	}
	// Zero seconds still renders the seconds tail.
	if got := Seconds2Time(0); got != "0 秒" {
		t.Errorf("Seconds2Time(0)=%q, want \"0 秒\"", got)
	}
}

// TestGenerateMessageID covers both the success and the malformed-account error
// branch of the SMTP Message-ID builder.
func TestGenerateMessageID(t *testing.T) {
	prev := SMTPFrom
	t.Cleanup(func() { SMTPFrom = prev })

	SMTPFrom = "noreply@lurus.cn"
	id, err := generateMessageID()
	if err != nil {
		t.Fatalf("generateMessageID: %v", err)
	}
	if !strings.HasSuffix(id, "@lurus.cn>") || !strings.HasPrefix(id, "<") {
		t.Errorf("message id malformed: %q", id)
	}

	SMTPFrom = "no-at-sign"
	if _, err := generateMessageID(); err == nil {
		t.Error("a From without a domain must error")
	}
}

// TestSendSms_Guards covers the two fail-safe guard branches: SMS disabled and
// SMS enabled-but-client-uninitialized must both surface an error rather than
// silently claiming a message was sent.
func TestSendSms_Guards(t *testing.T) {
	prevEnabled, prevClient := SMSEnabled, SmsClient
	t.Cleanup(func() { SMSEnabled, SmsClient = prevEnabled, prevClient })

	SMSEnabled = false
	if err := SendSms("13800138000", "login", "{}"); err == nil {
		t.Error("disabled SMS must error")
	}

	SMSEnabled = true
	SmsClient = nil
	if err := SendSms("13800138000", "login", "{}"); err == nil {
		t.Error("enabled SMS with a nil client must error")
	}
}

// TestSendEmail_Guards covers SendEmail's two early fail-safe branches without a
// live SMTP server: a From/Account with no domain fails the Message-ID build,
// and an unconfigured server surfaces the "not configured" error.
func TestSendEmail_Guards(t *testing.T) {
	pFrom, pAcct, pSrv := SMTPFrom, SMTPAccount, SMTPServer
	t.Cleanup(func() { SMTPFrom, SMTPAccount, SMTPServer = pFrom, pAcct, pSrv })

	// No From and no Account ⇒ generateMessageID rejects the malformed account.
	SMTPFrom, SMTPAccount, SMTPServer = "", "", ""
	if err := SendEmail("s", "to@x.com", "body"); err == nil {
		t.Error("SendEmail must error when the SMTP account has no domain")
	}

	// Valid From but server+account unconfigured ⇒ explicit not-configured error.
	SMTPFrom, SMTPAccount, SMTPServer = "noreply@lurus.cn", "", ""
	if err := SendEmail("s", "to@x.com", "body"); err == nil {
		t.Error("SendEmail must error when SMTP is unconfigured")
	}
}
