package cli

import (
	"os"
	"testing"
)

func TestTerminalKeysDoNotTreatSpecialKeysAsEscape(t *testing.T) {
	tests := []struct{ name, seq, want string }{
		{"upCSI", "\x1b[A", "up"},
		{"downCSI", "\x1b[B", "down"},
		{"rightCSI", "\x1b[C", "right"},
		{"leftCSI", "\x1b[D", "left"},
		{"upSS3", "\x1bOA", "up"},
		{"f1", "\x1bOP", ""},
		{"f2", "\x1bOQ", ""},
		{"f3", "\x1bOR", ""},
		{"f4", "\x1bOS", ""},
		{"f5", "\x1b[15~", ""},
		{"f12", "\x1b[24~", ""},
		{"insert", "\x1b[2~", ""},
		{"delete", "\x1b[3~", ""},
		{"pageUp", "\x1b[5~", ""},
		{"pageDown", "\x1b[6~", ""},
		{"shiftTab", "\x1b[Z", ""},
		{"controlUp", "\x1b[1;5A", ""},
		{"altQ", "\x1bq", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			_, err = w.Write([]byte(tt.seq + "x"))
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			keys := &terminalKeys{file: r}
			got, err := keys.readKey()
			if err != nil || got != tt.want {
				t.Fatalf("readKey() = %q, %v; want %q", got, err, tt.want)
			}
			got, err = keys.readKey()
			if err != nil || got != "x" {
				t.Fatalf("special-key trailing bytes leaked: %q, %v", got, err)
			}
		})
	}
}

func TestTerminalKeysBareEscape(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_, err = w.Write([]byte{27})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := (&terminalKeys{file: r}).readKey()
	if err != nil || got != "esc" {
		t.Fatalf("got %q, %v", got, err)
	}
}
