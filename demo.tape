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

# --- Set up a throwaway demo repo with a remote (hidden from the recording) --
Hide
Type "cd $(mktemp -d) && git init -q --bare origin.git && git init -q work && cd work"  Enter
Type "git config user.email demo@example.com && git config user.name Demo && git remote add origin ../origin.git"  Enter
Type `c(){ printf '%s\n' "$2" >> "$1" && git add "$1" && git commit -qm "$3"; }`  Enter
Type "c README.md hello 'feat: initial commit'; c parser.go p1 'feat(parser): tokenize input'; c parser.go p2 'fix(parser): handle empty input'"  Enter
Type "c pager.go g1 'feat(pager): scrollable output'; c pager.go g2 'fix(pager): off-by-one at last page'; c README.md usage 'docs: usage section'"  Enter
Type "git push -qu origin HEAD 2>/dev/null; printf 'world\n' >> README.md && printf 'release notes\n' > NOTES.md; clear"  Enter
Show

# --- Launcher menu: run gito with no arguments ------------------------------
Type "gito"      Enter
Sleep 1500ms
Down
Sleep 500ms
Down
Sleep 1s
Escape
Sleep 600ms

# --- status: stage changes ----------------------------------------------------
Type "gito status"   Enter
Sleep 1500ms
Type "a"
Sleep 1500ms
Escape
Sleep 500ms
Hide
Type "git commit -qm 'docs: release notes' && clear"  Enter
Show

# --- log: search history with / ---------------------------------------------
Type "gito log"      Enter
Sleep 1500ms
Type "/"
Sleep 400ms
Type "fix"
Sleep 1500ms
Enter
Sleep 400ms
Enter
Sleep 2s
Type "q"
Sleep 500ms
Escape
Sleep 400ms
Escape
Sleep 600ms

# --- remote: push the new commit (asks first, never forces) -----------------
Type "gito remote"   Enter
Sleep 1500ms
Type "p"
Sleep 1500ms
Type "y"
Sleep 2s
Type "q"
Sleep 1500ms
Escape
Sleep 1s
