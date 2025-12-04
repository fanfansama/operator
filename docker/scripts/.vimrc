" Activer la détection du type de fichier
filetype on
filetype indent on
filetype plugin on

" Spécifique au YAML
autocmd FileType yaml setlocal tabstop=2 shiftwidth=2 expandtab
autocmd FileType yaml setlocal autoindent
autocmd FileType yaml setlocal foldmethod=indent
autocmd FileType yaml setlocal foldlevel=99

" Affichage des espaces et indentations (utile pour YAML)
set list
set listchars=tab:»·,trail:·

" Améliorer la navigation avec indentation
set expandtab
set tabstop=2
set shiftwidth=2
set softtabstop=2
set autoindent
set smartindent
set backspace=indent,eol,start

" Activer la coloration syntaxique
syntax on

" Numérotation des lignes
set number
set relativenumber

