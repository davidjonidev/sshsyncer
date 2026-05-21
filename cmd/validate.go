package cmd

import (
	"fmt"
	"strconv"
)

func nonEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	return nil
}

func isPort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("must be 1-65535")
	}
	return nil
}
