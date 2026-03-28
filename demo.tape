# Loadout TUI Demo
# Requires: loadout configured and pointed at a repo with skills

Output demo.gif
Set Shell zsh
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"

# Launch TUI off-screen
Hide
Type "./loadout --user"
Enter
Sleep 2s
Show

# Browse skills
Sleep 1.5s
Type "j"
Sleep 1.2s
Type "j"
Sleep 1.2s
Type "k"
Sleep 1.2s

# Switch to details pane and scroll
Type "l"
Sleep 1.5s
Type "j"
Sleep 1s
Type "j"
Sleep 1s
Type "j"
Sleep 1s

# Switch back to skills pane
Type "h"
Sleep 1.5s

# Filter skills
Type "/"
Sleep 800ms
Type "review"
Sleep 2s

# Clear filter
Escape
Sleep 1.5s

# Equip a skill
Type "c"
Sleep 2s

# Unequip
Type "c"
Sleep 2s

# Open help
Type "?"
Sleep 2.5s

# Scroll help
Type "j"
Sleep 800ms
Type "j"
Sleep 800ms
Type "j"
Sleep 800ms

# Close help
Escape
Sleep 2s
