# Instruções do projeto — lofi & chill CLI

## Escopo e referência

- Este projeto é uma aplicação CLI com TUI em Go para estudo e concentração, com Pomodoro, rádio lofi e mixer de sons ambientes.
- A aplicação web de referência está em `/home/rafaelnacle/projects/codex/lofi-chill`. Consulte sua implementação para entender comportamentos e reutilizar os recursos autorizados, preservando seus arquivos.
- Mantenha a versão Go neste projeto separado. Não transporte a arquitetura web, React, Vite, APIs de navegador ou configuração de GitHub Pages para a CLI.
- Use soluções locais e simples, sem backend, banco de dados ou serviços pagos sem necessidade e solicitação explícita.
- Bibliotecas de TUI e áudio ainda não foram escolhidas. Avalie opções gratuitas, abertas e mantidas; explique benefícios, dependências de sistema e limitações antes de adotar uma solução.
- Antes de implementar o áudio, investigue a viabilidade do rádio e do mixer simultâneos. Não presuma que o comportamento do player web pode ser reproduzido diretamente no terminal.

## Desenvolvimento em Go

- Prefira soluções simples, código legível e mudanças focadas na tarefa. Não refatore código funcional sem relação com o pedido.
- Siga a estrutura e as convenções existentes; ao iniciar o projeto, adote uma estrutura enxuta, sem pacotes, interfaces ou camadas especulativas.
- Adicione dependências somente quando trouxerem benefício claro. Prefira a biblioteca padrão quando ela atender bem ao requisito.
- Separe as regras do timer, a persistência e o processamento de áudio da renderização da TUI o suficiente para permitir testes independentes.
- Não bloqueie o loop de eventos com rede, decodificação de áudio ou operações demoradas. Controle o ciclo de vida de goroutines e processos externos, com cancelamento e encerramento previsíveis.
- Trate erros com mensagens úteis e contexto. Falhas de áudio ou rede não devem impedir o uso do Pomodoro.
- Feche arquivos, dispositivos e processos ao sair; restaure o estado do terminal também em caminhos de erro e interrupção.
- Use `gofmt` e execute `go test ./...`, `go vet ./...` e a compilação relevante quando houver código Go. Siga as ferramentas já configuradas, sem adicionar ou atualizar tooling sem necessidade.
- Para alterações de concorrência, execute testes com `-race` quando suportado. Teste regras de tempo com relógio controlável, evitando esperas reais longas.

## Interface de terminal

- Preserve a identidade synthwave: roxo, rosa e ciano, com referências a VHS/videocassete e estética old-school. Legibilidade e uso vêm primeiro.
- Destaque o timer e mantenha os controles principais, o rádio e o mixer fáceis de acessar, evitando rolagem desnecessária.
- Todas as ações essenciais devem funcionar por teclado. Exiba atalhos e ajuda; deixe claros o foco de navegação, o modo selecionado e os estados de reprodução.
- Adapte o layout ao tamanho e ao redimensionamento do terminal. Em terminais pequenos, priorize controles essenciais e evite cortes que impeçam o uso.
- Ofereça modo foco para reduzir distrações. Evite animações excessivas, cintilação e redesenhos desnecessários.
- Não comunique estados apenas por cor. Combine texto, símbolos e destaque; ofereça alternativas legíveis quando cores ou caracteres especiais não forem suportados.
- Mantenha logs e mensagens de diagnóstico fora da área de renderização interativa. Ajuda e erros da CLI devem ser claros, com códigos de saída apropriados.
- Verifique navegação, atalhos, redimensionamento, contraste, saída e restauração do terminal após mudanças na TUI.

## Pomodoro: comportamentos obrigatórios

- Modos: foco, pausa curta e pausa longa. Durações padrão de 25, 5 e 15 minutos, configuráveis entre 1 e 120 minutos.
- Permita iniciar, pausar, retomar e resetar. Apenas um countdown pode rodar por vez.
- Trocar de modo pausa e preserva o progresso de cada sessão. Ao voltar a uma sessão iniciada, ofereça retomar (`Resume`).
- Alterar a duração configurada não apaga nem modifica o progresso de uma sessão iniciada. A nova duração vale para sessões novas ou após Reset.
- Calcule o tempo por tempo decorrido real/deadline, usando o relógio monotônico quando aplicável. Não dependa da quantidade de ticks recebidos.
- Ao concluir foco, incremente o contador diário uma única vez e prepare uma pausa; a cada quatro focos concluídos, ofereça pausa longa.
- Pausas começam manualmente. Ao terminar uma pausa, volte ao foco e preserve eventual progresso de foco pausado.
- Pausar, resetar ou trocar de modo não conta como conclusão. Trate explicitamente a troca de modo no instante do término para evitar perda ou duplicação de conclusão.
- Reinicie o contador diário na mudança do dia local, inclusive se o programa permanecer aberto.
- Emita apenas um aviso sonoro por conclusão, inclusive nas pausas, respeitando a preferência de som.
- Cubra com testes preservação entre modos, retomada, mudanças de duração, conclusão única, troca no término, retorno ao foco pausado e mudança de dia.

## Áudio

### Aviso de conclusão

- Reproduza um aviso suave equivalente ao da web: duas notas senoidais de `523.25 Hz` e `659.25 Hz`, segunda nota iniciando `0.24 s` depois, ataque aproximado de `0.035 s` e duração de cerca de `1.3 s` por nota, com decaimento.
- Consulte `hooks/use-timer-chime.ts` da referência para verificar o ganho salvo; não assuma se o valor é `0.055` ou `0.07`. Ajuste a equivalência ao mecanismo de áudio escolhido.
- Disponibilize controles para ativar/desativar e testar o aviso.

### Rádio lofi

Preserve as estações escolhidas pelo usuário:

| Estação | URL |
| --- | --- |
| Lofi Girl — beats to study | https://www.youtube.com/watch?v=rFZHOHl-L8A |
| steezyasfuck — hip hop beats | https://www.youtube.com/watch?v=rPjez8z61rI |
| Lofi Girl — synthwave | https://www.youtube.com/watch?v=4xDzrJKXOOY |
| Lofi Girl — beats to sleep/chill | https://www.youtube.com/watch?v=JD-kMIpDfnY |

- Mantenha os controles dentro do aplicativo. Abrir outra página ao acionar Play não atende à experiência desejada.
- Essas URLs são páginas do YouTube, não streams de áudio diretos. Verifique a solução de reprodução, suas condições de uso, licença, manutenção e dependências antes da escolha.
- Não baixe para armazenamento nem empacote as transmissões no aplicativo; não presuma autorização para redistribuição.
- Trate indisponibilidade de transmissões, ausência de dependências e falhas de reprodução com mensagens úteis e possibilidade de recuperação.
- Não substitua estações silenciosamente. Explique limitações concretas quando não for possível reproduzir a experiência pedida.

### Mixer — Set the mood

- Cinco canais independentes, capazes de tocar simultaneamente e junto com o rádio: Rainfall, Brown noise, Ocean hush, Birdsong e Fireplace.
- Reutilize as gravações reais de chuva, pássaros e lareira em `public/audio/rain.mp3`, `public/audio/birds.mp3` e `public/audio/fire.mp3` da referência. Não substitua essas gravações por síntese.
- Preserve os créditos e as fontes CC0 documentados em `public/audio/CREDITS.md` ao copiar ou distribuir esses recursos.
- Brown noise e Ocean hush são sintetizados. Consulte `lib/ambient.ts` para os comportamentos de referência.
- Ofereça volume individual de 0 a 100% e play/pause geral do mixer. Preserve transições suaves e loops sem estalos perceptíveis.
- Valores iniciais: chuva em 35%; demais canais em 0%. Persista volumes, mas não inicie reprodução automaticamente ao abrir o programa.
- Ao adicionar canais, preserve preferências existentes e inicialize os novos em 0%.
- Verifique carregamento, descarte de recursos, migração de preferências e reprodução simultânea. Faça verificação auditiva quando disponível; informe quando o ambiente impedir essa validação.

## Persistência local

- Use arquivos em diretórios apropriados de configuração/dados do usuário, respeitando as convenções do sistema operacional. Não grave preferências no diretório de instalação ou no repositório.
- Persista preferências e contagem diária com sua data local. Valide valores carregados e trate arquivos ausentes, antigos ou inválidos sem derrubar o app.
- Faça gravações seguras, evitando arquivos parcialmente escritos, e preserve configurações compatíveis ao migrar formatos.
- A referência preserva sessões ao trocar de modo, mas não após recarregar. Restaurar sessões depois de fechar a CLI é uma decisão nova: deixe o comportamento explícito antes de implementá-lo.
- Não armazene segredos em arquivos comuns de preferências nem retome áudio automaticamente na inicialização.

## Segurança

- Nunca inclua ou exponha chaves, tokens, senhas ou outros segredos em código, logs, capturas, documentação ou commits.
- Nunca exponha o conteúdo de arquivos `.env` nem versione arquivos `.env` com segredos. Use variáveis de ambiente quando configuração sensível for necessária.
- Valide entradas, caminhos e configurações externas. Ao executar players ou utilitários, passe argumentos diretamente, evitando interpolação em shell.
- Não enfraqueça controles de segurança para facilitar a implementação.

## Git e distribuição

- Use sempre Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:` etc.
- Faça um commit separado e focado para cada mudança importante. Crie branches quando necessário e mantenha o histórico limpo e compreensível.
- Somente o usuário faz push. Nunca execute `git push` nem envie commits por outra ferramenta.
- Não publique releases, binários, pacotes ou outras distribuições sem solicitação do usuário.

## README

- Mantenha o `README.md` conciso e voltado ao usuário: finalidade, tecnologias, instalação, configuração e comandos.
- Documente dependências de sistema necessárias ao áudio, plataformas efetivamente suportadas e atalhos essenciais quando relevante.
- Documente nomes e finalidade de variáveis de ambiente necessárias, nunca valores secretos reais.
- Não acrescente histórico de desenvolvimento, roadmap, explicações da conversa, arquitetura, detalhes internos ou conteúdo genérico sem pedido explícito.
- Atualize o README somente quando a mudança afetar o que o usuário precisa saber para instalar, configurar ou usar o app.

## Consulta à implementação web

Todos os caminhos abaixo são relativos ao projeto de referência:

- `AGENTS.md`: regras gerais que originaram estas diretrizes.
- `lib/timer.ts` e `tests/timer.test.ts`: regras do Pomodoro e regressões esperadas.
- `hooks/use-timer-chime.ts`: aviso de conclusão.
- `lib/ambient.ts` e `tests/ambient.test.ts`: síntese, loops, carregamento, descarte e preferências.
- `components/ambient-mixer.tsx`: controles do mixer.
- `lib/radio.ts` e `components/radio-player.tsx`: estações e comportamento do rádio.
- `src/App.tsx`: integração e controles.
- `public/audio/CREDITS.md`: procedência e licença das gravações.
- `docs/screenshot.png`: referência visual, adaptada às limitações do terminal.
