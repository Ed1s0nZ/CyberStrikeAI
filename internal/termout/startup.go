package termout

import (
	"fmt"
	"os"
	"strings"
)

// StartupWebUIOptions configures the startup Web UI banner.
type StartupWebUIOptions struct {
	Scheme       string
	Port         int
	SelfSigned   bool
	HTTPRedirect bool
}

// PrintConfigCreated prints a short notice when config.yaml is bootstrapped.
func PrintConfigCreated() {
	s := New(os.Stdout)
	s.Println("")
	s.Println(s.Green("✔ ") + s.Bold("已创建 config.yaml") + s.Dim("（来自 config.example.yaml）"))
	s.BlankLine()
}

// PrintStartupWebUI prints a colored startup banner for the Web UI.
func PrintStartupWebUI(opts StartupWebUIOptions) {
	s := New(os.Stdout)
	scheme := opts.Scheme
	if scheme == "" {
		scheme = "http"
	}
	port := opts.Port
	if port <= 0 {
		port = 8080
	}
	url := fmt.Sprintf("%s://127.0.0.1:%d/", scheme, port)

	s.BlankLine()
	s.Println(s.Bold(s.Cyan("CYBERSTRIKE AI")) + s.Dim("  /  secure workspace"))
	s.Println(s.Dim(strings.Repeat("─", 60)))
	s.Println(s.Green("● ONLINE") + "   " + s.Bold(s.White(url)))
	if opts.SelfSigned {
		s.Println(s.Dim("  TLS      ") + s.Yellow("self-signed") + s.Dim(" · accept the browser warning once"))
	}
	if opts.HTTPRedirect {
		s.Println(s.Dim("  Redirect ") + fmt.Sprintf("http://127.0.0.1:%d/ → HTTPS", port))
	}
	s.BlankLine()
}

// PrintBootstrapAdminCredentials prints the initial admin password banner.
func PrintBootstrapAdminCredentials(password string) {
	password = strings.TrimSpace(password)
	if password == "" {
		return
	}

	s := New(os.Stdout)
	s.Println(s.Bold(s.Yellow("ADMIN SETUP REQUIRED")))
	s.Println(s.Dim(strings.Repeat("─", 60)))
	s.Println(s.Dim("  Username  ") + s.Bold(s.White("admin")))
	s.Println(s.Dim("  Password  ") + s.Bold(s.Yellow(password)))
	s.BlankLine()
	s.Println(s.Yellow("  ! ") + s.White("Store this password securely. It is shown only once."))
	s.Println(s.Dim("    Change it in Settings immediately after signing in."))
	s.BlankLine()
}
