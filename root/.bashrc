# .bashrc

# User specific aliases and functions

alias rm='rm -i'
alias cp='cp -i'
alias mv='mv -i'

# Source global definitions
if [ -f /etc/bashrc ]; then
	. /etc/bashrc
fi

[[ -s "/root/.gvm/scripts/gvm" ]] && source "/root/.gvm/scripts/gvm"

gvm use go1.12.6
GO111MODULE=on
