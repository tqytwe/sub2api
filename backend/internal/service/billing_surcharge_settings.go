package service

import (
	"context"
	"strconv"
	"strings"
)

func (s *SettingService) GetBillingSurchargeConfig(ctx context.Context) BillingSurchargeConfig {
	if s == nil || s.settingRepo == nil {
		return BillingSurchargeConfig{}
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyBillingSurchargeEnabled,
		SettingKeyBillingSurchargeMode,
		SettingKeyBillingSurchargeValue,
	})
	if err != nil {
		return BillingSurchargeConfig{}
	}
	value, _ := strconv.ParseFloat(strings.TrimSpace(values[SettingKeyBillingSurchargeValue]), 64)
	if value < 0 {
		value = 0
	}
	return BillingSurchargeConfig{
		Enabled: strings.EqualFold(strings.TrimSpace(values[SettingKeyBillingSurchargeEnabled]), "true"),
		Mode:    NormalizeBillingSurchargeMode(values[SettingKeyBillingSurchargeMode]),
		Value:   value,
	}
}
