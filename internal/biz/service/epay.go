package service

import (
	"github.com/QuantumNous/lurus-api/internal/pkg/setting/operation_setting"
	"github.com/QuantumNous/lurus-api/internal/pkg/setting/system_setting"
)

func GetCallbackAddress() string {
	if operation_setting.CustomCallbackAddress == "" {
		return system_setting.ServerAddress
	}
	return operation_setting.CustomCallbackAddress
}
