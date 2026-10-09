package config

import "testing"

func TestValidateJump(t *testing.T) {
	valid := []string{"", "bastion", "dev@bastion.example.com", "jump1,jump2:2222", "dev@jump1:2200,ops@jump2:2222", "[2001:db8::1]", "dev@[2001:db8::1]:2200", "ssh_config_alias"}
	for _, value := range valid {
		if err := ValidateJump(value); err != nil {
			t.Errorf("Valid jump %q rejected: %v", value, err)
		}
	}
	invalid := []string{" ", " jump1", "jump1 ", "jump1,", ",jump2", "jump1,,jump2", "-oProxyCommand=evil", "alice@-badhost", "foo;touch /tmp/x", "jump1\nForceCommand", "jump1:0", "jump1:65536", "jump1:999999", "jump1:abcd", "x@@y", "bad/host"}
	for _, value := range invalid {
		if err := ValidateJump(value); err == nil {
			t.Errorf("Invalid jump %q accepted", value)
		}
	}
}

func TestLegacyJumpMigration(t *testing.T) {
	before := Target{Host: "private.internal", User: "ubuntu", Port: 2200, Jump: "sysadmin@bastion:2222"}
	endpoints := EndpointList(before)
	if len(endpoints) != 1 || endpoints[0].Endpoint.Jump != before.Jump {
		t.Fatalf("legacy target did not propagate jump: %+v", endpoints)
	}
	after, err := PromoteLegacyTarget(before, "private")
	if err != nil {
		t.Fatal(err)
	}
	if after.Jump != "" || after.Endpoints["private"].Jump != before.Jump {
		t.Fatalf("legacy migration lost jump or left a stale setting: %+v", after)
	}
}
