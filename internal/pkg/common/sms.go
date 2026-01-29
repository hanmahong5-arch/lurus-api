package common

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
)

// SMS Configuration Variables
var (
	SMSEnabled         = false
	SMSAccessKeyId     = ""
	SMSAccessKeySecret = ""
	SMSSignName        = ""
	SMSRegionId        = "cn-hangzhou"
	SmsClient          *dysmsapi.Client
)

// SMS Template Codes - configured in Aliyun SMS console
const (
	SmsTemplateLogin     = "login"
	SmsTemplateRegister  = "register"
	SmsTemplateReset     = "reset"
	SmsTemplateBindPhone = "bind"
)

// SMS Rate Limit Configuration
var (
	SMSRateLimitPerPhone    = 1               // 1 request per phone per minute
	SMSRateLimitPerIP       = 10              // 10 requests per IP per hour
	SMSCodeExpiration       = 5 * time.Minute // Code expires in 5 minutes
	SMSRateLimitPhoneTTL    = 60              // seconds
	SMSRateLimitIPTTL       = 3600            // seconds
)

// InitSmsClient initializes the Aliyun SMS client
func InitSmsClient() error {
	if !SMSEnabled {
		SysLog("SMS service is disabled")
		return nil
	}

	if SMSAccessKeyId == "" || SMSAccessKeySecret == "" {
		return errors.New("SMS AccessKeyId or AccessKeySecret is not configured")
	}

	config := &openapi.Config{
		AccessKeyId:     tea.String(SMSAccessKeyId),
		AccessKeySecret: tea.String(SMSAccessKeySecret),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}

	client, err := dysmsapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create SMS client: %v", err)
	}
	SmsClient = client
	SysLog("SMS client initialized successfully")
	return nil
}

// SendSms sends an SMS message using Aliyun SMS service
func SendSms(phone, templateCode, templateParam string) error {
	if !SMSEnabled {
		return errors.New("SMS service is disabled")
	}
	if SmsClient == nil {
		return errors.New("SMS client not initialized")
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone),
		SignName:      tea.String(SMSSignName),
		TemplateCode:  tea.String(templateCode),
		TemplateParam: tea.String(templateParam),
	}

	response, err := SmsClient.SendSms(request)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %v", err)
	}

	if response.Body == nil || response.Body.Code == nil || *response.Body.Code != "OK" {
		msg := "unknown error"
		if response.Body != nil && response.Body.Message != nil {
			msg = *response.Body.Message
		}
		return fmt.Errorf("SMS send failed: %s", msg)
	}

	SysLog(fmt.Sprintf("SMS sent successfully to %s", maskPhone(phone)))
	return nil
}

// GenerateSmsCode generates a random 6-digit numeric verification code for SMS
func GenerateSmsCode() string {
	code := ""
	for i := 0; i < 6; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code
}

// IsValidChinesePhone validates Chinese mobile phone number
func IsValidChinesePhone(phone string) bool {
	// Chinese mobile phone: 11 digits starting with 1
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// maskPhone masks phone number for logging (13800138000 -> 138****8000)
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// GetSMSTemplateCode returns the actual template code from Aliyun SMS console
// You need to replace these with your actual template codes
func GetSMSTemplateCode(purpose string) string {
	// These should be configured in the admin panel or config file
	// For now, return the purpose as placeholder
	templateCodeMap := map[string]string{
		SmsTemplateLogin:     OptionMap["SMSTemplateLogin"],
		SmsTemplateRegister:  OptionMap["SMSTemplateRegister"],
		SmsTemplateReset:     OptionMap["SMSTemplateReset"],
		SmsTemplateBindPhone: OptionMap["SMSTemplateBind"],
	}

	if code, ok := templateCodeMap[purpose]; ok && code != "" {
		return code
	}

	// Fallback: use a default template code if configured
	if defaultCode := OptionMap["SMSTemplateDefault"]; defaultCode != "" {
		return defaultCode
	}

	// If no template configured, return empty (will cause send to fail)
	return ""
}

// BuildSMSTemplateParam builds the template parameter JSON string
func BuildSMSTemplateParam(code string) string {
	return fmt.Sprintf(`{"code":"%s"}`, code)
}
