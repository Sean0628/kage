# VHS demo script for kage (影)
# Run: vhs demo.tape
# Requires: vhs (brew install vhs), kage, tmux, git

Output assets/demo.gif
Set Width 1200
Set Height 700
Set FontSize 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms

# Start kage dashboard
Type "kage"
Sleep 500ms
Enter
Sleep 3s

# Navigate the project list
Sleep 1s
Down
Sleep 800ms
Down
Sleep 800ms
Up
Sleep 1s

# Create a new chore branch
Type "n"
Sleep 1s
Type "chore-add-readme"
Sleep 500ms
Enter
Sleep 3s

# Return to dashboard (Ctrl+a K)
Ctrl+a
Sleep 300ms
Type "K"
Sleep 2s

# Delete the feature
Type "d"
Sleep 1s
Type "y"
Sleep 2s
