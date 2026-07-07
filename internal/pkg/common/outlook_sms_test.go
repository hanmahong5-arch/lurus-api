package common

import (
	"testing"
)

func TestOutlookAuth(t *testing.T) {
	auth := LoginAuth("me@outlook.com", "pw").(*outlookAuth)

	// Start advertises the LOGIN mechanism.
	proto, resp, err := auth.Start(nil)
	if err != nil || proto != "LOGIN" || len(resp) != 0 {
		t.Errorf("Start = (%q,%v,%v)", proto, resp, err)
	}

	// Next replies with username then password on the server prompts.
	if b, err := auth.Next([]byte("Username:"), true); err != nil || string(b) != "me@outlook.com" {
		t.Errorf("username step = (%q,%v)", b, err)
	}
	if b, err := auth.Next([]byte("Password:"), true); err != nil || string(b) != "pw" {
		t.Errorf("password step = (%q,%v)", b, err)
	}
	// Unknown prompt is an error.
	if _, err := auth.Next([]byte("Captcha:"), true); err == nil {
		t.Error("unknown prompt should error")
	}
	// more=false yields no data.
	if b, err := auth.Next(nil, false); err != nil || b != nil {
		t.Errorf("non-more step = (%v,%v)", b, err)
	}
}

func TestIsOutlookServer(t *testing.T) {
	if !isOutlookServer("smtp.outlook.com") {
		t.Error("outlook host should match")
	}
	if !isOutlookServer("mail.onmicrosoft.com") {
		t.Error("onmicrosoft host should match")
	}
	if isOutlookServer("smtp.gmail.com") {
		t.Error("gmail host should not match")
	}
}

func TestInitSmsClient(t *testing.T) {
	origEnabled := SMSEnabled
	origID, origSecret := SMSAccessKeyId, SMSAccessKeySecret
	origClient := SmsClient
	t.Cleanup(func() {
		SMSEnabled, SMSAccessKeyId, SMSAccessKeySecret, SmsClient = origEnabled, origID, origSecret, origClient
	})

	// Disabled: no-op success.
	SMSEnabled = false
	if err := InitSmsClient(); err != nil {
		t.Errorf("disabled init should succeed: %v", err)
	}

	// Enabled but unconfigured: error.
	SMSEnabled = true
	SMSAccessKeyId = ""
	SMSAccessKeySecret = ""
	if err := InitSmsClient(); err == nil {
		t.Error("enabled+unconfigured should error")
	}

	// Enabled + configured: constructs a client (no network call at init).
	SMSAccessKeyId = "ak"
	SMSAccessKeySecret = "sk"
	if err := InitSmsClient(); err != nil {
		t.Errorf("configured init should succeed: %v", err)
	}
	if SmsClient == nil {
		t.Error("SmsClient should be set after configured init")
	}
}

func TestSendSms_GuardPaths(t *testing.T) {
	origEnabled := SMSEnabled
	origClient := SmsClient
	t.Cleanup(func() { SMSEnabled, SmsClient = origEnabled, origClient })

	SMSEnabled = false
	if err := SendSms("13800000000", "login", "{}"); err == nil {
		t.Error("disabled SMS should error")
	}

	SMSEnabled = true
	SmsClient = nil
	if err := SendSms("13800000000", "login", "{}"); err == nil {
		t.Error("uninitialized client should error")
	}
}
