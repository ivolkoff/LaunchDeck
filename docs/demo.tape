# vhs tape for the LaunchDeck demo GIF.
# Regenerate with:  vhs docs/demo.tape   (run from the repo root; needs ./launchdeck built)
Output docs/demo.gif

Set Shell "zsh"
Set FontSize 13
Set Width 1120
Set Height 660
Set Padding 12
Set Theme "Dracula"

Type "./launchdeck"
Enter
Sleep 2.5s

# Live regex filter on the sidebar (opened with f)
Type "f"
Sleep 600ms
Type "com.apple"
Sleep 2s
Escape
Sleep 800ms

# Move the selection, then flip through the detail tabs
Down@250ms 4
Sleep 800ms
Type "2"
Sleep 1.6s
Type "3"
Sleep 1.6s
Type "1"
Sleep 1s

# Action picker for the selected service
Type "a"
Sleep 1.8s
Escape
Sleep 700ms

Type "q"
Sleep 800ms
