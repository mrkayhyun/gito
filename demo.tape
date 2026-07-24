# gito demo tape
# ---------------------------------------------------------------------------
# Renders the demo GIF shown at the top of the README.
#
#   1. Install VHS:   https://github.com/charmbracelet/vhs
#   2. Build gito:    go build -o gito . && cp gito "$(go env GOPATH)/bin/"  (or put it on PATH)
#   3. Render:        vhs demo.tape          # writes docs/demo.gif
#
# Tweak the Sleep durations if a screen needs more/less time on your machine.
# ---------------------------------------------------------------------------

Output docs/demo.gif

Require gito

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

# --- Set up a throwaway demo repo (hidden from the recording) ---------------
Hide
Type "cd $(mktemp -d)"                                                    Enter
Type "git init -q && git config user.email demo@example.com && git config user.name Demo"  Enter
Type "printf 'hello\n' > README.md && git add README.md && git commit -qm 'feat: initial commit'"  Enter
Type "printf 'world\n' >> README.md && printf 'release notes\n' > NOTES.md"  Enter
Type "clear"                                                              Enter
Show

# --- Launcher menu: run gito with no arguments ------------------------------
Type "gito"      Enter
Sleep 2s
Down
Sleep 600ms
Down
Sleep 1500ms
Escape
Sleep 700ms

# --- status: stage and review changes ---------------------------------------
Type "gito status"   Enter
Sleep 2s
Space
Sleep 900ms
Type "a"
Sleep 1500ms
Escape
Sleep 700ms

# --- log: browse history and open a commit ----------------------------------
Type "gito log"      Enter
Sleep 2s
Enter
Sleep 2s
Type "q"
Sleep 700ms
Escape
Sleep 1200ms
