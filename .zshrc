export PATH=/usr/local/bin:/System/Cryptexes/App/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/opt/homebrew/bin:/opt/homebrew/bin

# Added by LM Studio CLI (lms)
export PATH="$PATH:/Users/sjaconette/.lmstudio/bin"

# Force gcloud/gsutil to use Python 3.12 — Python 3.13 segfaults on macOS
# during DNS resolution (getaddrinfo fork → _os_log_preferences_refresh SIGSEGV)
export CLOUDSDK_PYTHON=/usr/local/bin/python3.12

# The next line updates PATH for the Google Cloud SDK.
if [ -f '/Users/sjaconette/Downloads/google-cloud-sdk/path.zsh.inc' ]; then . '/Users/sjaconette/Downloads/google-cloud-sdk/path.zsh.inc'; fi

# The next line enables shell command completion for gcloud.
if [ -f '/Users/sjaconette/Downloads/google-cloud-sdk/completion.zsh.inc' ]; then . '/Users/sjaconette/Downloads/google-cloud-sdk/completion.zsh.inc'; fi


# BEGIN opam configuration
# This is useful if you're using opam as it adds:
#   - the correct directories to the PATH
#   - auto-completion for the opam binary
# This section can be safely removed at any time if needed.
[[ ! -r '/Users/sjaconette/.opam/opam-init/init.zsh' ]] || source '/Users/sjaconette/.opam/opam-init/init.zsh' > /dev/null 2> /dev/null
# END opam configuration

. "$HOME/.local/bin/env"

# Added by Antigravity
export PATH="/Users/sjaconette/.antigravity/antigravity/bin:$PATH"
export PATH=$PATH:~/go/bin

# DeepFlow CLI
export PATH="/Users/sjaconette/terminal-tasks/bin:$PATH"

# Move large caches to external drive when available
_ext="/Volumes/My Passport/.caches"
if [ -d "$_ext" ] && [ "${DEV_WORKSPACE_MANAGED:-}" != "1" ]; then
    export UV_CACHE_DIR="$_ext/uv"
    export PIP_CACHE_DIR="$_ext/pip"
    export PUPPETEER_CACHE_DIR="$_ext/puppeteer"
    export HOMEBREW_CACHE="$_ext/homebrew"
fi
unset _ext

# Auto-load fooszone R2 credentials (private file, chmod 600)
[ -f "$HOME/.config/fooszone/r2.env" ] && source "$HOME/.config/fooszone/r2.env"

# dotfiles repo (bare, work-tree=$HOME) — set up 2026-07-03
alias dot='git --git-dir=$HOME/.dotfiles.git --work-tree=$HOME'

autoload -Uz compinit && compinit
if command -v wt >/dev/null 2>&1; then eval "$(command wt config shell init zsh)"; fi
