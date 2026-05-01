package utils

import "strings"

func GetCommandFromMessage(message string) *string {
	cleanedMessage := strings.TrimSpace(message)
	if len(cleanedMessage) <= 1 || cleanedMessage[0] != '!' {
		return nil
	}

	parts := strings.Fields(cleanedMessage)
	cmd := strings.ToLower(parts[0][1:])
	return &cmd
}
