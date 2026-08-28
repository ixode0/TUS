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

// scanLnWithoutEcho prompts the user for input without echoing the characters typed.
func scanLnWithoutEcho(s string) (string, error) {
	fmt.Print(s)
	// Try to use term.ReadPassword if stdin is a TTY, fallback to buffered reader for non-interactive (pipe/fifo)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		input, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err == nil {
			fmt.Println()
			return strings.TrimSpace(string(input)), nil
		}
	}
	// Fallback: read line from stdin (also check /tmp/tg_code file for automation)
	if codeEnv := os.Getenv("TG_CODE"); codeEnv != "" {
		fmt.Println("[using TG_CODE env]")
		return strings.TrimSpace(codeEnv), nil
	}
	// allow feeding via file for bot forwarding
	if data, err := os.ReadFile("/tmp/tg_code.txt"); err == nil {
		if c := strings.TrimSpace(string(data)); c != "" {
			// consume it
			_ = os.Remove("/tmp/tg_code.txt")
			fmt.Println("[using /tmp/tg_code.txt]")
			return c, nil
		}
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
