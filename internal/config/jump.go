package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// jumpHop accepts the documented OpenSSH ProxyJump host syntax: an SSH config
// alias, DNS name, IPv4 address, or bracketed IPv6 address; optionally with
// username and port. Reject control characters and SSH option injection.
var jumpHop = regexp.MustCompile(`^(?:[A-Za-z0-9_][A-Za-z0-9_.-]*@)?(?:[A-Za-z0-9_][A-Za-z0-9_.-]*|\[[0-9A-Fa-f:.%]+\])(?::([0-9]{1,5}))?$`)

// ValidateJump checks a comma-separated series of SSH ProxyJump hops.
// An empty value means use OpenSSH's default routing / ~/.ssh/config.
func ValidateJump(value string) error {
	if value == "" {
		return nil
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("invalid SSH jump %q: remove surrounding whitespace", value)
	}
	for _, hop := range strings.Split(value, ",") {
		matches := jumpHop.FindStringSubmatch(hop)
		if matches == nil {
			return fmt.Errorf("invalid SSH jump hop %q: expected [user@]host[:port] or [user@][IPv6][:port]", hop)
		}
		if matches[1] != "" {
			port, err := strconv.Atoi(matches[1])
			if err != nil || port == 0 || port > 65535 {
				return fmt.Errorf("invalid SSH jump port in %q (must be 1-65535)", hop)
			}
		}
	}
	return nil
}
