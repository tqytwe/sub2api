package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

type IPRiskAlertEmailNotifier struct {
	settings     SettingRepository
	notification *NotificationEmailService
}

func NewIPRiskAlertEmailNotifier(
	settings SettingRepository,
	notification *NotificationEmailService,
) *IPRiskAlertEmailNotifier {
	return &IPRiskAlertEmailNotifier{settings: settings, notification: notification}
}

func (n *IPRiskAlertEmailNotifier) NotifyIPRiskLevel(
	ctx context.Context,
	riskCase *IPRiskCase,
	assessment IPRiskAssessment,
	previousLevel RiskLevel,
) error {
	if n == nil || n.settings == nil || n.notification == nil || riskCase == nil {
		return nil
	}
	raw, err := n.settings.GetValue(ctx, SettingKeyAccountQuotaNotifyEmails)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil
		}
		return err
	}
	recipients := filterVerifiedEmails(ParseNotifyEmails(raw))
	if len(recipients) == 0 {
		return nil
	}

	signalCodes := make([]string, 0, len(assessment.Signals))
	for _, signal := range assessment.Signals {
		signalCodes = append(signalCodes, string(signal.Code))
	}
	var sendErr error
	for _, recipient := range recipients {
		err := n.notification.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventIPRiskAlert,
			Locale:         "zh",
			RecipientEmail: recipient,
			RecipientName:  "Administrator",
			SourceType:     "ip_risk_case",
			SourceID:       strconv.FormatInt(riskCase.ID, 10),
			ReminderKey:    string(assessment.Level),
			Variables: map[string]string{
				"case_id":             strconv.FormatInt(riskCase.ID, 10),
				"primary_ip":          riskCase.PrimaryIP,
				"primary_network":     riskCase.PrimaryNetwork,
				"risk_score":          strconv.Itoa(assessment.Score),
				"risk_level":          string(assessment.Level),
				"previous_risk_level": string(previousLevel),
				"signal_codes":        strings.Join(signalCodes, ", "),
				"detected_at":         riskCase.LastDetectedAt.UTC().Format(time.RFC3339),
			},
		})
		if err != nil {
			sendErr = errors.Join(sendErr, err)
		}
	}
	return sendErr
}
