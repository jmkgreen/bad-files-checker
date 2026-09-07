package app

import (
	"fmt"
	"syscall"
)

const (
	ioprioWhoProcess = 1
	ioprioClassBE    = 2
	ioprioClassIdle  = 3
)

func applyNice(nice int) error {
	if nice == 0 {
		return nil
	}
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, nice); err != nil {
		return fmt.Errorf("apply nice %d: %w", nice, err)
	}
	return nil
}

func applyIonice(class string, level int) error {
	classValue, err := ioniceClassValue(class)
	if err != nil {
		return err
	}

	value := classValue << 13
	if classValue == ioprioClassBE {
		value |= level
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOPRIO_SET, uintptr(ioprioWhoProcess), 0, uintptr(value))
	if errno != 0 {
		return fmt.Errorf("apply ionice %s/%d: %w", class, level, errno)
	}
	return nil
}

func ioniceClassValue(class string) (int, error) {
	switch class {
	case "best-effort":
		return ioprioClassBE, nil
	case "idle":
		return ioprioClassIdle, nil
	default:
		return 0, fmt.Errorf("unsupported ionice class %q", class)
	}
}
