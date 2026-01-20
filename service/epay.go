package service

import (
	"github.com/QuantumNous/lurus-api/setting/operation_setting"
	"github.com/QuantumNous/lurus-api/setting/system_setting"
)

func GetCallbackAddress() string {
	if operation_setting.CustomCallbackAddress == "" {
		return system_setting.ServerAddress
	}
	return operation_setting.CustomCallbackAddress
}
