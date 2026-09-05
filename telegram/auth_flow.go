package telegram

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

type noSignUp struct{}

func (n noSignUp) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("not implemented")
}

func (n noSignUp) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

type SimpleAuthFlow struct {
	noSignUp    // Prevent signup
	PhoneNumber string
}

func (s SimpleAuthFlow) Phone(context.Context) (string, error) {
	return s.PhoneNumber, nil
}

func (s SimpleAuthFlow) Password(context.Context) (string, error) {
	password, err := scanLnWithoutEcho("Enter your password: ")
	if err != nil {
		return "", err
	}

	return string(password), nil
}

func (s SimpleAuthFlow) Code(context.Context, *tg.AuthSentCode) (string, error) {
	code, err := scanLnWithoutEcho("Enter the code you received to your telegram account: ")
	if err != nil {
		return "", err
	}

	return string(code), nil
}

// codeFilePath can be overridden for tests.
var codeFilePath = "/tmp/tg_code.txt"

// scanLnWithoutEcho prompts the user for input without echoing the characters typed.
func scanLnWithoutEcho(s string) (string, error) {
	fmt.Print(s)
	// Try to use term.ReadPassword if stdin is a TTY, fallback to buffered reader for non-interactive (pipe/fifo)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		input, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err == nil {
			fmt.Println()
			if c := strings.TrimSpace(string(input)); c != "" {
				return c, nil
			}
		}
	}
	// Automation: env first (note: visible in `ps e`, prefer file/stdin for shared hosts).
	if codeEnv := os.Getenv("TG_CODE"); codeEnv != "" {
		fmt.Println("[using TG_CODE env]")
		return strings.TrimSpace(codeEnv), nil
	}
	// allow feeding via file for bot forwarding (consumed after reading).
	// Lstat BEFORE ReadFile: validate the path (no symlink, safe perms)
	// prior to trusting its contents (avoids TOCTOU read-then-check).
	if st, statErr := os.Lstat(codeFilePath); statErr == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(os.Stderr, "warning: %s is a symlink, refusing to read code file (symlink attack risk)\n", codeFilePath)
		} else {
			if perm := st.Mode().Perm(); perm&0o077 != 0 {
				fmt.Fprintf(os.Stderr, "warning: %s is world/group-readable (mode %04o); hardening to 0600\n", codeFilePath, perm)
				if chErr := os.Chmod(codeFilePath, 0o600); chErr != nil {
					fmt.Fprintf(os.Stderr, "warning: chmod 600 %s failed: %v\n", codeFilePath, chErr)
				}
			}
			if data, err := os.ReadFile(codeFilePath); err == nil {
				if c := strings.TrimSpace(string(data)); c != "" {
					// consume it
					_ = os.Remove(codeFilePath)
					fmt.Println("[using code file]")
					return c, nil
				}
			}
		}
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
