# lofi & chill

Pomodoro, rádio lofi e cinco sons ambientes em uma TUI synthwave para terminal, escrita em Go com Bubble Tea. Preferências locais, sem conta ou backend.

## Instalação

Requer Go 1.27.1 ou superior. A integração de áudio foi feita para Linux e macOS; o rádio usa sockets Unix e não oferece suporte a Windows nesta versão.

```sh
go build -o bin/lofi-chill ./cmd/lofi-chill
./bin/lofi-chill
```

Instale **mpv** para mixer e aviso sonoro, e **yt-dlp** para o rádio. Ambos precisam estar no `PATH`. O Pomodoro funciona sem essas dependências.

```sh
# Fedora
sudo dnf install mpv yt-dlp
# macOS com Homebrew
brew install mpv yt-dlp

./bin/lofi-chill --doctor
./bin/lofi-chill --help
```

O rádio reproduz as quatro estações escolhidas do YouTube dentro do app, sem navegador e sem salvar transmissões. A reprodução depende da disponibilidade das estações e da compatibilidade de mpv/yt-dlp com o YouTube; bloqueios de rede, região ou autenticação podem impedir o acesso. Não requer chave de API.

## Controles

| Tecla | Ação |
| --- | --- |
| `espaço` | Iniciar, pausar ou retomar timer |
| `1` / `2` / `3` | Foco / pausa curta / pausa longa |
| `r` | Reset da sessão atual |
| `-` / `+` | Duração do modo, entre 1 e 120 minutos |
| `Tab` / `Shift+Tab` | Selecionar próximo painel / painel anterior |
| `Enter` | Ativar a ação selecionada |
| `f` | Modo foco |
| `p` | Tocar/parar rádio |
| `[` / `]` | Estação anterior/próxima |
| `,` / `.` | Volume do rádio |
| `m` | Play/pause geral do mixer |
| `↑` / `↓` ou `k` / `j` | Selecionar item dentro do painel ativo |
| `←` / `→` ou `h` / `l` | Ajustar modo, duração, estação ou volume selecionado |
| `c` / `t` | Ativar/desativar aviso / testar som |
| `?` | Ajuda |
| `Esc` | Fechar ajuda ou sair do modo foco |
| `q` / `Ctrl+C` | Sair |

O painel ativo tem um marcador `>` e borda destacada. O timer permanece visível no cabeçalho, e o rodapé mostra os atalhos do item selecionado. O rádio indica conexão, reprodução, parada ou falha; `Tocando` só aparece após confirmação do player.

## Configuração

Durações iniciais: 25/5/15 minutos. Trocar de modo pausa e preserva a sessão; alterar a duração só afeta sessões novas ou resetadas. Pausas começam manualmente. A cada quatro focos concluídos, o app prepara uma pausa longa.

Preferências e contagem diária ficam em `lofi-chill/config.json` dentro do diretório de configuração do usuário: `$XDG_CONFIG_HOME` ou `~/.config` no Linux, `~/Library/Application Support` no macOS. A contagem reinicia no dia local. Sessões não são restauradas após sair, e o áudio sempre inicia parado. Chuva começa configurada em 35%; demais canais em 0%.

Gravações CC0 e seus créditos: [fontes dos áudios](internal/audio/assets/CREDITS.md). Brown noise, Ocean hush e aviso são sintetizados localmente.
