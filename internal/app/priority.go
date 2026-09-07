package app

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultNice        = 10
	defaultIoniceClass = "best-effort"
	defaultIoniceLevel = 7
)

type runtimeSettings struct {
	Nice        int
	IoniceClass string
	IoniceLevel int
}

func parseRuntimeSettings(nice int, ioniceClass string, ioniceLevel int) (runtimeSettings, error) {
	ioniceClass = strings.ToLower(strings.TrimSpace(ioniceClass))
	if ioniceClass == "" {
		return runtimeSettings{}, errors.New("ionice class cannot be empty")
	}

	if nice < 0 || nice > 19 {
		return runtimeSettings{}, fmt.Errorf("nice must be between 0 and 19, got %d", nice)
	}

	settings := runtimeSettings{
		Nice:        nice,
		IoniceClass: ioniceClass,
		IoniceLevel: ioniceLevel,
	}

	switch ioniceClass {
	case "none":
		settings.IoniceLevel = 0
	case "idle":
		settings.IoniceLevel = 0
	case "best-effort", "be":
		settings.IoniceClass = "best-effort"
		if ioniceLevel < 0 || ioniceLevel > 7 {
			return runtimeSettings{}, fmt.Errorf("ionice level must be between 0 and 7 for best-effort class, got %d", ioniceLevel)
		}
	default:
		return runtimeSettings{}, fmt.Errorf("unsupported ionice class %q; use none, idle, or best-effort", ioniceClass)
	}

	return settings, nil
}

func applyRuntimeSettings(settings runtimeSettings) error {
	if err := applyNice(settings.Nice); err != nil {
		return err
	}
	if settings.IoniceClass == "none" {
		return nil
	}

	return applyIonice(settings.IoniceClass, settings.IoniceLevel)
}
