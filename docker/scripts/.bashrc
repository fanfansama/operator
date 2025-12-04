alias k=kubectl
alias h=helm
alias hf=helmfile

alias vi=vim
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
alias ls='ls --color=auto'
alias dir='dir --color=auto'
alias vdir='vdir --color=auto'
alias grep='grep --color=auto'

source <(kubectl completion bash)
complete -F __start_kubectl k

source <(helm completion bash)
complete -o default -F __start_helm h

#source <(helmfile completion bash)
#complete -o default -F __start_helmfile hf

export TERM=xterm-256color
export LANG=C.UTF-8
export LC_ALL=C.UTF-8
export EDITOR=vim
export VISUAL=vim
export HISTCONTROL=ignoredups:erasedups  # no duplicate entries
export HISTSIZE=100

if [[ -f $HOME/.env ]]; then
    source $HOME/.env
fi

if [ -f $HOME/.vimrc ]; then
  export VIMINIT='source $HOME/.vimrc'
fi

if [ -f $HOME/.bash_aliases ]; then
  source $HOME/.bash_aliases
fi