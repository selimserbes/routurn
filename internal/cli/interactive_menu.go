package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// interactiveMenu is shared by exec, update, and result. Only display labels
// live here; callers retain all workflow validation and execution authority.
type interactiveMenuItem struct {
	Label  string
	Detail string
}
type interactiveMenu struct {
	Heading        string
	Items          []interactiveMenuItem
	SearchItems    []interactiveMenuItem // optional project-wide search pool
	Shortcuts      []string              // key + description, e.g. "h Groups"
	Searching      bool                  // text-entry mode; normal task navigation has no input cursor
	ConfirmHint    string                // action-specific Enter explanation; defaults to "Confirm selection"
	TextInputLabel string                // optional inline input mode (e.g. "Command" or "Path")
	EnteringText   bool                  // render the text-entry view instead of the task picker
	TextInputValue string                // current editor contents (display only)
}
type menuSelection struct {
	Index         int // -1 for shortcuts/cancel
	Key           string
	FromSearchAll bool
	Text          string // input entered in the inline editor when Key is "e"
}

// terminalKeys activates single-key navigation only on a real, attached TTY.
// On Unix, stty is used without new Go dependencies; Windows and unsupported
// terminals retain the deterministic line-based selector.
type terminalKeys struct {
	file  *os.File
	saved string
}

func openTerminalKeys(cmd *cobra.Command) *terminalKeys {
	if !pickerIsInteractiveTerminal(cmd) || runtime.GOOS == "windows" {
		return nil
	}
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return nil
	}
	old, err := stty(f, "-g")
	if err != nil {
		return nil
	}
	// Keep output CR/LF translation enabled: raw mode otherwise causes
	// successive menu lines to start at the previous line's end column.
	if _, err = stty(f, "raw", "-echo", "min", "0", "time", "1", "opost", "onlcr"); err != nil {
		return nil
	}
	return &terminalKeys{file: f, saved: strings.TrimSpace(old)}
}
func stty(f *os.File, args ...string) (string, error) {
	c := exec.Command("stty", args...)
	c.Stdin = f
	b, e := c.CombinedOutput()
	return string(b), e
}
func (t *terminalKeys) close() {
	if t != nil {
		_, _ = stty(t.file, t.saved)
	}
}

// readKey returns one terminal input event. Raw-mode reads may return zero
// bytes due to VTIME; the loop tolerates that without busy-spinning.
func (t *terminalKeys) readByteOnce() (byte, bool, error) {
	var b [1]byte
	n, e := t.file.Read(b[:])
	if n == 1 {
		return b[0], true, nil
	}
	if e == io.EOF { // VMIN=0/VTIME timeout is not an input EOF.
		return 0, false, nil
	}
	return 0, false, e
}
func (t *terminalKeys) readByte() (byte, error) {
	for {
		b, ok, e := t.readByteOnce()
		if ok {
			return b, nil
		}
		if e != nil {
			return 0, e
		}
	}
}
func (t *terminalKeys) readKey() (string, error) {
	b, e := t.readByte()
	if e != nil {
		return "", e
	}
	if b != 27 {
		if b >= utf8.RuneSelf {
			buf := []byte{b}
			for !utf8.FullRune(buf) && len(buf) < utf8.UTFMax {
				next, e := t.readByte()
				if e != nil {
					return "", e
				}
				buf = append(buf, next)
			}
			if utf8.Valid(buf) {
				return string(buf), nil
			}
			return "", nil
		}
		return string([]byte{b}), nil
	}
	// A bare Escape means Back, but many special keys begin with Escape.
	// Never treat an unrecognized / incomplete escape sequence as Back.
	next, ok, e := t.readByteOnce()
	if e != nil {
		return "", e
	}
	if !ok {
		return "esc", nil
	}
	if next != '[' && next != 'O' {
		// Alt+key and other unsupported sequences are harmless.
		return "", nil
	}
	// SS3 sequences: arrows or F1-F4 (ESC O P/Q/R/S).
	if next == 'O' {
		third, ok, e := t.readByteOnce()
		if e != nil {
			return "", e
		}
		if !ok {
			return "", nil
		}
		return arrowEscapeKey(third), nil
	}
	// CSI sequences: arrows, Insert, Delete, Home, PgUp/PgDn,
	// F5-F12, and modifier variants. Consume the ENTIRE sequence so its
	// trailing bytes do not become menu shortcuts or command text.
	var seq []byte
	for i := 0; i < 32; i++ {
		ch, ok, e := t.readByteOnce()
		if e != nil {
			return "", e
		}
		if !ok {
			return "", nil
		}
		seq = append(seq, ch)
		if ch >= 0x40 && ch <= 0x7e { // CSI final byte
			if len(seq) == 1 {
				return arrowEscapeKey(ch), nil
			}
			return "", nil
		}
	}
	return "", nil
}

func arrowEscapeKey(ch byte) string {
	switch ch {
	case 'A':
		return "up"
	case 'B':
		return "down"
	case 'C':
		return "right"
	case 'D':
		return "left"
	}
	return ""
}

func normalizeMenuShortcut(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "q", "quit", "esc":
		return "q"
	case "n", "next":
		return "n"
	case "p", "prev", "previous":
		return "p"
	case "s", "search", "/":
		return "/"
	case "a", "all":
		return "a"
	case "h", "groups":
		return "h"
	case "u", "ungrouped":
		return "u"
	case "g", "home", "back":
		return "g"
	case "e", "custom":
		return "e"
	case "b", "browse":
		return "b"
	case "m", "managed":
		return "m"
	case "c", "clear":
		return "c"
	}
	return ""
}
func menuSearchIndices(items []interactiveMenuItem, query string) []int {
	out := make([]int, 0, len(items))
	q := strings.ToLower(strings.TrimSpace(query))
	for i, v := range items {
		if q == "" || strings.Contains(strings.ToLower(v.Label+" "+v.Detail), q) {
			out = append(out, i)
		}
	}
	return out
}
func menuPickPage(indexes []int, page int) ([]int, int, int) {
	totalPages := 1
	if len(indexes) > 0 {
		totalPages = (len(indexes) + 9) / 10
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	a := page * 10
	b := a + 10
	if b > len(indexes) {
		b = len(indexes)
	}
	return indexes[a:b], page, totalPages
}
func menuShow(out io.Writer, title string, menu interactiveMenu, shown []int, page, pages, focus int, filter, notice string, color, raw bool) {
	width := pickerDisplayWidth()
	if raw {
		// Navigation hides the cursor; search and the inline editor show it.
		if menu.Searching || menu.EnteringText {
			fmt.Fprint(out, "\x1b[?25h")
		} else {
			fmt.Fprint(out, "\x1b[?25l")
		}
		fmt.Fprint(out, "\x1b[H\x1b[2J")
	}
	fmt.Fprintln(out, "Routurn · "+title)
	fmt.Fprintln(out, strings.Repeat("─", width))
	fmt.Fprintln(out, menu.Heading)
	if pages > 1 {
		fmt.Fprintf(out, "Page %d/%d\n", page+1, pages)
	}
	if filter != "" && !menu.Searching {
		fmt.Fprintf(out, "Search: %s\n", filter)
	}
	fmt.Fprintln(out)
	for i, idx := range shown {
		item := menu.Items[idx]
		marker := " "
		if raw && !menu.EnteringText && focus == i {
			marker = "❯"
		}
		key := strconv.Itoa(i + 1)
		if i == 9 {
			key = "0"
		}
		label := fmt.Sprintf("%s %s) %s", marker, key, item.Label)
		if item.Detail != "" {
			label += "  · " + item.Detail
		}
		// Do not silently truncate task identities: long names wrap naturally.
		switch {
		case raw && menu.EnteringText && color:
			// The task list remains visible, but keyboard focus belongs to
			// the editor in the footer rather than to a task.
			fmt.Fprintf(out, "\x1b[2m%s\x1b[0m\n", label)
		case raw && focus == i && color:
			fmt.Fprintf(out, "\x1b[1;96m%s\x1b[0m\n", label)
		default:
			fmt.Fprintln(out, label)
		}
	}
	if len(shown) == 0 {
		fmt.Fprintln(out, "  No matching items.")
	}
	fmt.Fprintln(out, "\n"+strings.Repeat("─", width))
	fmt.Fprintln(out, "Shortcuts")
	var opts []string
	if menu.EnteringText {
		// Only shortcuts that are valid while editing are shown.
		action := "Run command"
		if menu.TextInputLabel == "Path" {
			action = "Use path"
		}
		opts = []string{"Enter " + action, "Esc Back", "Backspace Delete", "Ctrl+U Clear"}
	} else {
		opts = append([]string{"/ Search"}, menu.Shortcuts...)
		if pages > 1 {
			opts = append(opts, "←→ Pages")
		}
		opts = append(opts, "q Exit")
		if raw {
			opts = append(opts, "↑↓ Navigate", "0-9 Highlight", "Enter Confirm", "Esc Back")
		}
	}
	pickerPrintOptions(out, width, opts, color)
	fmt.Fprintln(out)
	if notice != "" {
		fmt.Fprintln(out, "  "+notice)
	}
	if raw {
		if menu.EnteringText {
			label := menu.TextInputLabel
			if label == "" {
				label = "Input"
			}
			fmt.Fprintln(out, label+":")
			fmt.Fprint(out, "  ❯ "+menu.TextInputValue)
		} else if menu.Searching {
			fmt.Fprintln(out, "Search tasks (Enter: finish, Esc: cancel):")
			fmt.Fprint(out, "  / "+filter)
		} else if len(shown) > 0 {
			fmt.Fprintln(out, "Selected: "+menu.Items[shown[focus]].Label)
			hint := strings.TrimSpace(menu.ConfirmHint)
			if hint == "" {
				hint = "Confirm selection"
			}
			fmt.Fprintln(out, "[Enter] "+hint)
		} else {
			fmt.Fprintln(out, "No task selected. Use / to search or Esc to go back.")
		}
		return
	}
	fmt.Fprintln(out, "Select a task or shortcut:")
	if color {
		fmt.Fprint(out, "  \x1b[1;96m❯\x1b[0m ")
	} else {
		fmt.Fprint(out, "  > ")
	}
}

// chooseInteractiveMenu returns an item index or an immediate navigation
// shortcut. Executing a task is never bound to a single digit: Enter confirms.
// All callers use this same component for consistent navigation and search.
func chooseInteractiveMenu(cmd *cobra.Command, title string, menu interactiveMenu) (menuSelection, error) {
	terminal := openTerminalKeys(cmd)
	if terminal != nil {
		defer func() {
			terminal.close()
			// Never leave the shell with an invisible cursor.
			fmt.Fprint(cmd.OutOrStdout(), "\x1b[?25h\n")
		}()
	}
	raw := terminal != nil
	out := cmd.OutOrStdout()
	color := raw && pickerColorEnabled(cmd)
	query := ""
	page := 0
	focus := 0
	searchMode := false
	textMode := false
	textValue := ""
	notice := ""
	for {
		active := menu.Items
		fromGlobal := len(menu.SearchItems) > 0 && (searchMode || query != "")
		if fromGlobal {
			active = menu.SearchItems
		}
		matches := menuSearchIndices(active, query)
		shown, actual, pages := menuPickPage(matches, page)
		page = actual
		if focus >= len(shown) {
			focus = len(shown) - 1
		}
		if focus < 0 {
			focus = 0
		}
		renderMenu := menu
		renderMenu.Items = active
		renderMenu.Searching = searchMode
		renderMenu.EnteringText = textMode
		renderMenu.TextInputValue = textValue
		if fromGlobal {
			renderMenu.Heading = menu.Heading + "  /  Search all tasks"
		}
		menuShow(out, title, renderMenu, shown, page, pages, focus, query, notice, color, raw)
		notice = ""
		if !raw {
			line, e := readMenuLine(cmd.InOrStdin())
			if e != nil && line == "" {
				return menuSelection{}, fmt.Errorf("interactive selection needs terminal input: %w", e)
			}
			value := strings.TrimSpace(line)
			if strings.HasPrefix(value, "/") && len(value) > 1 {
				query = strings.TrimSpace(value[1:])
				page = 0
				focus = 0
				continue
			}
			if key := normalizeMenuShortcut(value); key != "" {
				switch key {
				case "/":
					fmt.Fprint(out, "Search: ")
					v, er := readMenuLine(cmd.InOrStdin())
					if er != nil && v == "" {
						return menuSelection{}, er
					}
					query = strings.TrimSpace(v)
					page = 0
					focus = 0
					continue
				case "n":
					page++
					continue
				case "p":
					page--
					continue
				case "c":
					query = ""
					page = 0
					continue
				default:
					return menuSelection{Index: -1, Key: key}, nil
				}
			}
			n, e := strconv.Atoi(value)
			if e != nil || n < 0 || n > 10 {
				notice = "Choose a listed number or shortcut."
				continue
			}
			if n == 0 {
				n = 10
			}
			if n > len(shown) {
				notice = "Choose a listed number or shortcut."
				continue
			}
			return menuSelection{Index: shown[n-1], FromSearchAll: fromGlobal}, nil
		}
		key, err := terminal.readKey()
		if err != nil {
			return menuSelection{}, err
		}
		if textMode {
			switch key {
			case "\r", "\n":
				value := strings.TrimSpace(textValue)
				if value != "" {
					return menuSelection{Index: -1, Key: "e", Text: value}, nil
				}
				notice = "Enter a value, or press Esc to return."
			case "esc", "\x03":
				textMode = false
				textValue = ""
				if menu.TextInputLabel == "Command" {
					notice = "Command cancelled; nothing was run."
				} else {
					notice = "Path entry cancelled; no update selected."
				}
			case "\x7f", "\b":
				if textValue != "" {
					_, sz := utf8.DecodeLastRuneInString(textValue)
					textValue = textValue[:len(textValue)-sz]
				}
			case "\x15":
				textValue = ""
			case "up", "down", "left", "right":
				// Navigation must not leak terminal escape sequences into input.
			default:
				if len(key) > 0 {
					r, _ := utf8.DecodeRuneInString(key)
					if unicode.IsPrint(r) && r != utf8.RuneError {
						textValue += key
					}
				}
			}
			continue
		}
		if searchMode {
			switch key {
			case "\r", "\n":
				searchMode = false
				continue
			case "esc":
				searchMode = false
				query = ""
				page, focus = 0, 0
				continue
			case "\x03":
				return menuSelection{Index: -1, Key: "q"}, nil
			case "\x7f", "\b":
				if query != "" {
					_, sz := utf8.DecodeLastRuneInString(query)
					query = query[:len(query)-sz]
				}
			default:
				if len(key) > 0 {
					r, _ := utf8.DecodeRuneInString(key)
					if unicode.IsPrint(r) {
						query += key
					}
				}
			}
			page = 0
			focus = 0
			continue
		}
		switch key {
		case "up":
			if focus > 0 {
				focus--
			} else if page > 0 {
				page--
				focus = 9
			}
		case "down":
			if focus+1 < len(shown) {
				focus++
			} else if page+1 < pages {
				page++
				focus = 0
			}
		case "right":
			if page+1 < pages {
				page++
				focus = 0
			}
		case "left":
			if page > 0 {
				page--
				focus = 0
			}
		case "\r", "\n":
			if len(shown) > 0 {
				return menuSelection{Index: shown[focus], FromSearchAll: fromGlobal}, nil
			}
		case "esc":
			return menuSelection{Index: -1, Key: "esc"}, nil
		case "\x03":
			return menuSelection{Index: -1, Key: "q"}, nil
		case "/":
			query = ""
			page = 0
			focus = 0
			searchMode = true
		case "\x7f", "\b":
			if query != "" {
				_, sz := utf8.DecodeLastRuneInString(query)
				query = query[:len(query)-sz]
				page = 0
				focus = 0
			}
		default:
			if key == "e" && menu.TextInputLabel != "" {
				textMode = true
				textValue = ""
				notice = ""
				continue
			}
			if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
				n := int(key[0] - '0')
				if n == 0 {
					n = 10
				}
				if n <= len(shown) {
					focus = n - 1
				}
				continue
			}
			if key == "q" {
				return menuSelection{Index: -1, Key: "q"}, nil
			}
			for _, option := range menu.Shortcuts {
				prefix, _, ok := strings.Cut(option, " ")
				if ok && key == prefix {
					return menuSelection{Index: -1, Key: key}, nil
				}
			}
		}
	}
}

// errTextInputCancelled means Esc (or Ctrl+C) cancelled text entry. Callers
// must return to their menu rather than executing an empty/custom command.
var errTextInputCancelled = errors.New("text entry cancelled")

// readMenuText is used for custom commands and archive paths. A normal cooked
// ReadString consumes arrow-key escape sequences as literal text. Read raw keys
// here instead, handle backspace, and allow Esc to return to the picker.
func readMenuText(cmd *cobra.Command, label string) (string, error) {
	terminal := openTerminalKeys(cmd)
	out := cmd.OutOrStdout()
	if terminal == nil {
		fmt.Fprint(out, label+": ")
		line, err := readMenuLine(cmd.InOrStdin())
		if err != nil && line == "" {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "\x1b" || line == "\x03" {
			return "", errTextInputCancelled
		}
		return line, nil
	}
	defer func() {
		terminal.close()
		fmt.Fprint(out, "\x1b[?25h\n")
	}()
	fmt.Fprint(out, "\x1b[?25h\n")
	buf := ""
	redraw := func() { fmt.Fprintf(out, "\r\x1b[2K%s: %s", label, buf) }
	redraw()
	for {
		key, err := terminal.readKey()
		if err != nil {
			return "", err
		}
		switch key {
		case "\r", "\n":
			fmt.Fprintln(out)
			return strings.TrimSpace(buf), nil
		case "esc", "\x03":
			fmt.Fprintln(out, "\nCancelled; returning to tasks.")
			return "", errTextInputCancelled
		case "\x7f", "\b":
			if len(buf) > 0 {
				_, n := utf8.DecodeLastRuneInString(buf)
				buf = buf[:len(buf)-n]
			}
		case "\x15": // Ctrl+U clears the typed line.
			buf = ""
		case "up", "down", "left", "right":
			// Arrow keys must never become literal ^[[C in command/path text.
			continue
		default:
			if len(key) > 0 {
				r, _ := utf8.DecodeRuneInString(key)
				if unicode.IsPrint(r) && r != utf8.RuneError {
					buf += key
				}
			}
		}
		redraw()
	}
}

// readMenuLine intentionally reads one byte at a time. Separate nested menus
// must not lose buffered bytes when non-interactive callers pipe several lines.
func readMenuLine(in io.Reader) (string, error) {
	var b [1]byte
	var s strings.Builder
	for {
		n, err := in.Read(b[:])
		if n > 0 {
			s.WriteByte(b[0])
			if b[0] == '\n' {
				return s.String(), nil
			}
		}
		if err != nil {
			if s.Len() > 0 && err == io.EOF {
				return s.String(), nil
			}
			return "", err
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
	}
}
